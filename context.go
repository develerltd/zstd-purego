package zstd

import (
	"fmt"
	"io"
	"unsafe"
)

// Reader implements an io.ReadCloser for reading and decompressing data
type Reader struct {
	zstd        *Zstd
	reader      io.Reader
	ctx         unsafe.Pointer
	buffer      []byte
	inBuffer    ZstdInBuffer
	outBuffer   ZstdOutBuffer
	readBuffer  []byte
	pos         int
	end         int
	streamEnded bool
	stream      unsafe.Pointer
}

// Read implements the io.Reader interface
func (r *Reader) Read(p []byte) (int, error) {
	// If we have data in the read buffer from a previous pass, use that first
	if r.pos < r.end {
		n := copy(p, r.readBuffer[r.pos:r.end])
		r.pos += n
		return n, nil
	}

	// If the stream was previously marked as ended and all buffered data is consumed
	if r.streamEnded {
		return 0, io.EOF
	}

	// Reset internal read buffer position; r.end will be set by decompressStream logic
	r.pos = 0
	r.end = 0

	// Loop to decompress data and fill r.readBuffer.
	// This loop continues as long as r.readBuffer is empty (r.end == 0) from this pass
	// and the Zstandard stream has not definitively ended for the current frame.
	for r.end == 0 && !r.streamEnded {
		// Ensure ZSTD stream context is initialized
		if r.stream == nil {
			r.stream = r.zstd.createDStream()
			if r.stream == nil {
				return 0, fmt.Errorf("failed to create decompression stream")
			}
			// r.readBuffer is initialized in NewReader
			if r.readBuffer == nil { // Safety check
				r.readBuffer = make([]byte, defaultReadBufferSize)
			}
		}

		// If ZSTD's input buffer (r.inBuffer) has been fully consumed, read more compressed data from the source.
		if r.inBuffer.Pos >= r.inBuffer.Size {
			nBytesFromSource, sourceReadErr := r.reader.Read(r.buffer) // r.buffer is a temporary store for compressed data

			if nBytesFromSource > 0 {
				r.inBuffer.Src = unsafe.Pointer(&r.buffer[0])
				r.inBuffer.Size = uint64(nBytesFromSource)
				r.inBuffer.Pos = 0
			} else {
				// No new bytes were read from the source.
				r.inBuffer.Src = nil // Ensure ZSTD sees an empty input buffer
				r.inBuffer.Size = 0
				r.inBuffer.Pos = 0
			}

			if sourceReadErr != nil {
				if sourceReadErr == io.EOF {
					// Source reader is at EOF. ZSTD_decompressStream will be called with an
					// empty input buffer. This is crucial for flushing ZSTD's internal buffers.
				} else {
					// A genuine error occurred while reading from the source.
					return 0, sourceReadErr // Propagate the error
				}
			}

			// If no bytes were read and no error (e.g., non-blocking read with no data),
			// we should break this inner loop. The outer Read logic will return 0, nil,
			// signaling the caller to try again.
			if nBytesFromSource == 0 && sourceReadErr == nil {
				break
			}
		}

		// Prepare the output buffer for ZSTD.
		// ZSTD will write decompressed data into r.readBuffer.
		r.outBuffer.Dst = unsafe.Pointer(&r.readBuffer[0])
		r.outBuffer.Size = uint64(len(r.readBuffer))
		r.outBuffer.Pos = 0 // ZSTD updates this to indicate how much was written.

		// Call the Zstandard C function to decompress the stream.
		zstdReturnHint := r.zstd.decompressStream(r.stream, &r.outBuffer, &r.inBuffer)

		if r.zstd.isError(zstdReturnHint) != 0 {
			r.streamEnded = true // Mark as ended on error to prevent further attempts.
			return 0, fmt.Errorf("zstd decompression error: %s", r.zstd.getErrorName(zstdReturnHint))
		}

		// r.end tracks how much valid decompressed data is in r.readBuffer.
		r.end = int(r.outBuffer.Pos)

		if zstdReturnHint == 0 {
			// A return hint of 0 means the current Zstandard frame is complete and fully flushed.
			r.streamEnded = true
			// Break this inner loop; r.readBuffer might contain the last chunk of data or be empty.
			break
		}

		// If zstdReturnHint > 0:
		// ZSTD indicates more processing is needed. It might have produced output (r.end > 0),
		// consumed input, or needs to be called again to flush internal state.
		// If ZSTD actually produced output (r.end > 0), we break this inner loop
		// to return the available data to the caller.
		if r.end > 0 {
			break
		}

		// If r.end == 0 (no output produced in this call) and zstdReturnHint > 0:
		// - If r.inBuffer was exhausted (r.inBuffer.Pos >= r.inBuffer.Size), the next iteration
		//   of this loop will attempt to read more from r.reader (or call ZSTD with empty
		//   input if source is EOF, for flushing). This is correct.
		// - If r.inBuffer was not exhausted, ZSTD needs to be called again with the remaining
		//   input in r.inBuffer. The loop continues.
		// - If source EOF'd, r.inBuffer is empty, ZSTD produces no output but hint > 0,
		//   this is the flushing scenario. The loop continues until hint becomes 0.
	} // End of inner loop for filling r.readBuffer

	// After the inner loop, r.readBuffer may have data, or r.streamEnded might be true.

	// If r.readBuffer is empty (r.end == 0) after trying to decompress:
	if r.end == 0 {
		if r.streamEnded {
			return 0, io.EOF // Stream ended and no data produced.
		}
		// No data produced, stream not marked as ended (e.g. underlying reader non-blocking and returned 0 bytes, 0 err)
		return 0, nil // Caller should try Read again.
	}

	// Copy decompressed data from r.readBuffer to the caller's buffer p.
	n := copy(p, r.readBuffer[r.pos:r.end])
	r.pos += n // Advance our position in r.readBuffer

	return n, nil
}

// Close implements the io.Closer interface
func (r *Reader) Close() error {
	if r.stream != nil {
		r.zstd.freeDStream(r.stream)
		r.stream = nil
	}
	if r.ctx != nil {
		r.zstd.freeDCtx(r.ctx)
		r.ctx = nil
	}
	return nil
}

// Writer implements an io.WriteCloser for compressing and writing data
type Writer struct {
	zstd      *Zstd
	writer    io.Writer
	ctx       unsafe.Pointer
	level     int
	buffer    []byte
	inBuffer  ZstdInBuffer
	outBuffer ZstdOutBuffer
	stream    unsafe.Pointer
}

// Write implements the io.Writer interface
func (w *Writer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	// Initialize stream if not already done
	if w.stream == nil {
		w.stream = w.zstd.createCStream()
		if w.stream == nil {
			return 0, fmt.Errorf("failed to create compression stream")
		}
	}

	// Set up input buffer
	w.inBuffer.Src = unsafe.Pointer(&p[0])
	w.inBuffer.Size = uint64(len(p))
	w.inBuffer.Pos = 0

	// Compress data in chunks if the output buffer is smaller than needed
	for w.inBuffer.Pos < w.inBuffer.Size {
		// Set up output buffer
		w.outBuffer.Dst = unsafe.Pointer(&w.buffer[0])
		w.outBuffer.Size = uint64(len(w.buffer))
		w.outBuffer.Pos = 0

		// Compress
		result := w.zstd.compressStream2(w.stream, &w.outBuffer, &w.inBuffer, EndContinue)

		// Check for errors
		if w.zstd.isError(result) != 0 {
			return int(w.inBuffer.Pos), fmt.Errorf("compression error: %s", w.zstd.getErrorName(result))
		}

		// Write compressed data
		if w.outBuffer.Pos > 0 {
			_, err := w.writer.Write(w.buffer[:w.outBuffer.Pos])
			if err != nil {
				return int(w.inBuffer.Pos), err
			}
		}
	}

	return len(p), nil
}

// Flush flushes any pending data to the underlying writer
func (w *Writer) Flush() error {
	if w.stream == nil {
		return nil
	}

	// Set up empty input buffer
	w.inBuffer.Src = nil
	w.inBuffer.Size = 0
	w.inBuffer.Pos = 0

	var result uint64

	// Flush until we're done
	for {
		// Set up output buffer
		w.outBuffer.Dst = unsafe.Pointer(&w.buffer[0])
		w.outBuffer.Size = uint64(len(w.buffer))
		w.outBuffer.Pos = 0

		// Perform flush operation
		result = w.zstd.compressStream2(w.stream, &w.outBuffer, &w.inBuffer, EndFlush)

		// Check for errors
		if w.zstd.isError(result) != 0 {
			return fmt.Errorf("flush error: %s", w.zstd.getErrorName(result))
		}

		// Write compressed data
		if w.outBuffer.Pos > 0 {
			_, err := w.writer.Write(w.buffer[:w.outBuffer.Pos])
			if err != nil {
				return err
			}
		}

		// If flush complete, break
		if result == 0 {
			break
		}
	}

	return nil
}

// Close implements the io.Closer interface
func (w *Writer) Close() error {
	defer func() {
		if w.ctx != nil {
			w.zstd.freeCCtx(w.ctx)
			w.ctx = nil
		}
	}()

	if w.stream == nil {
		return nil
	}

	// Set up empty input buffer
	w.inBuffer.Src = nil
	w.inBuffer.Size = 0
	w.inBuffer.Pos = 0

	var result uint64

	// End the stream and write pending data
	for {
		// Set up output buffer
		w.outBuffer.Dst = unsafe.Pointer(&w.buffer[0])
		w.outBuffer.Size = uint64(len(w.buffer))
		w.outBuffer.Pos = 0

		// End the stream
		result = w.zstd.compressStream2(w.stream, &w.outBuffer, &w.inBuffer, EndEnd)

		// Check for errors
		if w.zstd.isError(result) != 0 {
			w.zstd.freeCStream(w.stream)
			w.stream = nil
			return fmt.Errorf("close error: %s", w.zstd.getErrorName(result))
		}

		// Write compressed data
		if w.outBuffer.Pos > 0 {
			_, err := w.writer.Write(w.buffer[:w.outBuffer.Pos])
			if err != nil {
				w.zstd.freeCStream(w.stream)
				w.stream = nil
				return err
			}
		}

		// If end complete, break
		if result == 0 {
			break
		}
	}

	// Free the stream
	w.zstd.freeCStream(w.stream)
	w.stream = nil

	return nil
}
