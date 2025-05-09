// Package zstd provides Go bindings to the Zstandard (zstd) compression library
// using purego to avoid CGo dependencies.
//
// This package embeds the zstd shared libraries for supported platforms and extracts them
// at runtime, allowing for easy cross-compilation and deployment without external dependencies.
//
// Currently supported platforms:
// - Linux amd64 (glibc 2.17+)
// - macOS arm64 (Apple Silicon)
package zstd

import (
	"fmt"
	"io"
	"unsafe"
)

// Version returns the library version as an integer
func (z *Zstd) Version() uint32 {
	return z.versionNumber()
}

// VersionString returns the library version as a string (e.g., "1.5.5")
func (z *Zstd) VersionString() string {
	return z.versionString()
}

// CompressBound returns the maximum compressed size in the worst case scenario.
func (z *Zstd) CompressBound(srcSize int) int {
	return int(z.compressBound(uint64(srcSize)))
}

// Compress compresses the data from src and returns the compressed data.
// Level can be between 1 (fastest) and 22 (highest compression ratio).
func (z *Zstd) Compress(src []byte, level int) ([]byte, error) {
	if len(src) == 0 {
		return []byte{}, nil
	}

	dstCapacity := z.CompressBound(len(src))
	dst := make([]byte, dstCapacity)

	result := z.compress(
		unsafe.Pointer(&dst[0]),
		uint64(dstCapacity),
		unsafe.Pointer(&src[0]),
		uint64(len(src)),
		level,
	)

	if z.isError(result) != 0 {
		return nil, fmt.Errorf("zstd compression error: %s", z.getErrorName(result))
	}

	return dst[:result], nil
}

// Decompress decompresses the data from src and returns the decompressed data.
// The maxSize parameter limits the maximum size of the decompressed data to prevent
// decompression bombs. Use 0 for the library default max size.
func (z *Zstd) Decompress(src []byte, maxSize int) ([]byte, error) {
	if len(src) == 0 {
		return []byte{}, nil
	}

	// If maxSize is 0, use a reasonable default
	if maxSize <= 0 {
		// Estimate the decompressed size - zstd typically achieves around 2.5-3x compression ratio
		// Use a conservative estimation with a safety factor
		maxSize = len(src) * 5
		if maxSize < 1024 {
			maxSize = 1024 // Minimum reasonable size
		}
	}

	dst := make([]byte, maxSize)

	result := z.decompress(
		unsafe.Pointer(&dst[0]),
		uint64(maxSize),
		unsafe.Pointer(&src[0]),
		uint64(len(src)),
	)

	if z.isError(result) != 0 {
		return nil, fmt.Errorf("zstd decompression error: %s", z.getErrorName(result))
	}

	return dst[:result], nil
}

// NewReader creates an io.ReadCloser for decompressing data from the provided reader.
// It will read and decompress data on demand.
func (z *Zstd) NewReader(r io.Reader) io.ReadCloser {
	return &Reader{
		zstd:   z,
		reader: r,
		ctx:    z.createDCtx(),
		buffer: make([]byte, defaultReadBufferSize),
	}
}

// NewWriter creates an io.WriteCloser for compressing data to the provided writer.
// The compressed data will be written to the provided writer.
// The caller must call Close() when done to ensure all data is flushed.
func (z *Zstd) NewWriter(w io.Writer, level int) io.WriteCloser {
	return &Writer{
		zstd:   z,
		writer: w,
		ctx:    z.createCCtx(),
		level:  level,
		buffer: make([]byte, defaultWriteBufferSize),
	}
}

// Close releases all resources associated with the Zstd instance.
// After Close is called, the Zstd instance cannot be used anymore.
func (z *Zstd) Close() error {
	if z.handle == 0 {
		return nil // Already closed
	}

	err := z.closeLibrary()
	z.handle = 0
	return err
}
