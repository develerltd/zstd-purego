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
	// If we have data in the read buffer, use that first
	if r.pos < r.end {
		n := copy(p, r.readBuffer[r.pos:r.end])
		r.pos += n
		return n, nil
	}

	// If the stream has ended, return EOF
	if r.streamEnded {
		return 0, io.EOF
	}

	// Reset buffer positions
	r.pos = 0
	r.end = 0

	// Read compressed data from the source
	n, err := r.reader.Read(r.buffer)
	if err != nil && err != io.EOF {
		return 0, err
	}

	// If there's nothing to read, return EOF or error
	if n == 0 {
		return 0, err
	}

	// Decompress the data
	if r.stream == nil {
		// Initialize stream if not already done
		r.stream = r.zstd.createDStream()
		if r.stream == nil {
			return 0, fmt.Errorf("failed to create decompression stream")
		}

		// Initialize read buffer
		if r.readBuffer == nil {
			r.readBuffer = make([]byte, defaultReadBufferSize)
		}
	}

	// Set up input buffer
	r.inBuffer.Src = unsafe.Pointer(&r.buffer[0])
	r.inBuffer.Size = uint64(n)
	r.inBuffer.Pos = 0

	// Set up output buffer
	r.outBuffer.Dst = unsafe.Pointer(&r.readBuffer[0])
	r.outBuffer.Size = uint64(len(r.readBuffer))
	r.outBuffer.Pos = 0

	// Decompress
	result := r.zstd.decompressStream(r.stream, &r.outBuffer, &r.inBuffer)

	// Check for errors
	if r.zstd.isError(result) != 0 {
		return 0, fmt.Errorf("decompression error: %s", r.zstd.getErrorName(result))
	}

	// Update buffer positions
	r.end = int(r.outBuffer.Pos)

	// Check if we've reached the end of the stream
	if r.inBuffer.Pos == r.inBuffer.Size && err == io.EOF {
		r.streamEnded = true
	}

	// Return data
	n = copy(p, r.readBuffer[:r.end])
	r.pos += n

	return n, nil
}

// Close implements the io.Closer interface
func (r *Reader) Close() error {
	if r.stream != nil {
		r.zstd.freeDStream(r.stream)
		r.stream = nil
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
