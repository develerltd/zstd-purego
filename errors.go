package zstd

import (
	"fmt"
	"io"
)

// Error represents a Zstandard error
type Error struct {
	Code    uint64
	Message string
}

// Error implements the error interface
func (e *Error) Error() string {
	return fmt.Sprintf("zstd error: %s (code: %d)", e.Message, e.Code)
}

// IsError returns true if the code represents an error condition
func IsError(code uint64) bool {
	// According to zstd_errors.h, error codes start at 1 for specific errors, and 0 means OK (no error)
	return code > 0
}

// Common errors
var (
	ErrInvalidLevel    = fmt.Errorf("zstd: invalid compression level")
	ErrCompression     = fmt.Errorf("zstd: compression error")
	ErrDecompression   = fmt.Errorf("zstd: decompression error")
	ErrOutputTooSmall  = fmt.Errorf("zstd: output buffer too small")
	ErrInputTooLarge   = fmt.Errorf("zstd: input too large")
	ErrContextCreation = fmt.Errorf("zstd: failed to create context")
	ErrEmptyInput      = fmt.Errorf("zstd: empty input, nothing to compress")
	ErrMaxSizeExceeded = fmt.Errorf("zstd: maximum size exceeded")
	ErrUnsupported     = fmt.Errorf("zstd: unsupported platform")
	ErrAlreadyClosed   = fmt.Errorf("zstd: already closed")
)

// Reader for testing that always returns an error
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (int, error) {
	return 0, e.err
}

// Writer for testing that always returns an error
type errorWriter struct {
	err error
}

func (e *errorWriter) Write(p []byte) (int, error) {
	return 0, e.err
}

// NewErrorReader creates a reader that always returns the specified error
func NewErrorReader(err error) io.Reader {
	return &errorReader{err: err}
}

// NewErrorWriter creates a writer that always returns the specified error
func NewErrorWriter(err error) io.Writer {
	return &errorWriter{err: err}
}
