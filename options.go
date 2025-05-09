package zstd

// Constants defining Zstandard compression levels
const (
	// Fast compression levels (negative values)
	BestSpeed    = 1
	FastSpeed    = 1
	DefaultSpeed = 3

	// Regular compression levels
	DefaultCompression = 3
	BetterCompression  = 7
	BestCompression    = 19 // Highest practical level, very slow
	UltraCompression   = 22 // Maximum possible level
)

// Constants for stream operations
const (
	// End operation modes for compressStream2
	EndContinue = 0 // More data to come
	EndFlush    = 1 // Flush pending data
	EndEnd      = 2 // End the frame

	// Default buffer sizes
	defaultReadBufferSize  = 16 * 1024 // 16KB
	defaultWriteBufferSize = 32 * 1024 // 32KB
)

// Options contains configuration options for the Zstd compressor/decompressor
type Options struct {
	CompressionLevel  int   // Compression level (1-22, default 3)
	WindowSize        int   // Window size limit (0 = default)
	ReadBufferSize    int   // Read buffer size for streaming operations
	WriteBufferSize   int   // Write buffer size for streaming operations
	MaxDecompressSize int64 // Maximum size limit for decompression (0 = no limit)
}

// DefaultOptions returns the default compression options
func DefaultOptions() Options {
	return Options{
		CompressionLevel:  DefaultCompression,
		WindowSize:        0, // Use library default
		ReadBufferSize:    defaultReadBufferSize,
		WriteBufferSize:   defaultWriteBufferSize,
		MaxDecompressSize: 0, // No limit
	}
}

// FastOptions returns options optimized for speed
func FastOptions() Options {
	opts := DefaultOptions()
	opts.CompressionLevel = BestSpeed
	return opts
}

// BestOptions returns options optimized for compression ratio
func BestOptions() Options {
	opts := DefaultOptions()
	opts.CompressionLevel = BestCompression
	return opts
}
