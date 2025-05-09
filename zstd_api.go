package zstd

import "io"

// New creates a new Zstandard instance.
// It handles loading the appropriate library for the current platform.
// The returned instance should be closed with Close() when done.
func New() (*Zstd, error) {
	return loadLibrary()
}

// CompressLevel compresses the input data using the specified compression level.
// Level should be between 1 (fastest) and 22 (highest compression ratio).
func CompressLevel(src []byte, level int) ([]byte, error) {
	z, err := New()
	if err != nil {
		return nil, err
	}
	defer z.Close()

	return z.Compress(src, level)
}

// Compress compresses the input data using the default compression level (3).
func Compress(src []byte) ([]byte, error) {
	return CompressLevel(src, DefaultCompression)
}

// CompressFast compresses the input data using the fastest compression level (1).
func CompressFast(src []byte) ([]byte, error) {
	return CompressLevel(src, BestSpeed)
}

// CompressBest compresses the input data using a high compression level (19).
func CompressBest(src []byte) ([]byte, error) {
	return CompressLevel(src, BestCompression)
}

// Decompress decompresses the input data.
// The maxSize parameter limits the maximum size of the decompressed data to
// prevent decompression bombs. Use 0 for the library default max size.
func Decompress(src []byte, maxSize int) ([]byte, error) {
	z, err := New()
	if err != nil {
		return nil, err
	}
	defer z.Close()

	return z.Decompress(src, maxSize)
}

// NewReader creates an io.ReadCloser for decompressing data from the provided reader.
// The returned reader should be closed with Close() when done.
func NewReader(r io.Reader) (io.ReadCloser, error) {
	z, err := New()
	if err != nil {
		return nil, err
	}

	reader := z.NewReader(r)

	// We need to wrap the reader to handle closing the zstd instance
	return &readCloserWrapper{
		ReadCloser: reader,
		zstd:       z,
	}, nil
}

// NewWriter creates an io.WriteCloser for compressing data to the provided writer
// using the default compression level.
// The returned writer should be closed with Close() when done.
func NewWriter(w io.Writer) (io.WriteCloser, error) {
	return NewWriterLevel(w, DefaultCompression)
}

// NewWriterLevel creates an io.WriteCloser for compressing data to the provided writer
// using the specified compression level.
// The returned writer should be closed with Close() when done.
func NewWriterLevel(w io.Writer, level int) (io.WriteCloser, error) {
	z, err := New()
	if err != nil {
		return nil, err
	}

	writer := z.NewWriter(w, level)

	// We need to wrap the writer to handle closing the zstd instance
	return &writeCloserWrapper{
		WriteCloser: writer,
		zstd:        z,
	}, nil
}

// readCloserWrapper wraps a ReadCloser and also closes the zstd instance
type readCloserWrapper struct {
	io.ReadCloser
	zstd *Zstd
}

// Close closes both the reader and the zstd instance
func (r *readCloserWrapper) Close() error {
	err1 := r.ReadCloser.Close()
	err2 := r.zstd.Close()

	if err1 != nil {
		return err1
	}
	return err2
}

// writeCloserWrapper wraps a WriteCloser and also closes the zstd instance
type writeCloserWrapper struct {
	io.WriteCloser
	zstd *Zstd
}

// Close closes both the writer and the zstd instance
func (w *writeCloserWrapper) Close() error {
	err1 := w.WriteCloser.Close()
	err2 := w.zstd.Close()

	if err1 != nil {
		return err1
	}
	return err2
}
