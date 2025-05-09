package zstd

import (
	"fmt"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Dictionary represents a pre-trained compression dictionary
type Dictionary struct {
	zstd     *Zstd
	dictData []byte
	dictID   uint32
}

// RegisterDictionary registers additional functions for dictionary operations
func (z *Zstd) registerDictionaryFunctions() error {
	// Check if already registered
	if z.createCDict != nil {
		return nil
	}

	// Register dictionary API functions
	purego.RegisterLibFunc(&z.createCDict, z.handle, "ZSTD_createCDict")
	purego.RegisterLibFunc(&z.freeCDict, z.handle, "ZSTD_freeCDict")
	purego.RegisterLibFunc(&z.createDDict, z.handle, "ZSTD_createDDict")
	purego.RegisterLibFunc(&z.freeDDict, z.handle, "ZSTD_freeDDict")
	purego.RegisterLibFunc(&z.compressUsingCDict, z.handle, "ZSTD_compress_usingCDict")
	purego.RegisterLibFunc(&z.decompressUsingDDict, z.handle, "ZSTD_decompress_usingDDict")
	purego.RegisterLibFunc(&z.getDictID, z.handle, "ZSTD_getDictID_fromDict")

	return nil
}

// LoadDictionary loads a pre-trained dictionary for compression/decompression
func (z *Zstd) LoadDictionary(dictData []byte) (*Dictionary, error) {
	if len(dictData) == 0 {
		return nil, fmt.Errorf("empty dictionary data")
	}

	// Register dictionary functions if needed
	if err := z.registerDictionaryFunctions(); err != nil {
		return nil, err
	}

	// Get dictionary ID
	dictID := z.getDictID(unsafe.Pointer(&dictData[0]), uint64(len(dictData)))

	return &Dictionary{
		zstd:     z,
		dictData: dictData,
		dictID:   dictID,
	}, nil
}

// ID returns the dictionary ID
func (d *Dictionary) ID() uint32 {
	return d.dictID
}

// CompressUsingDict compresses data using the dictionary
func (z *Zstd) CompressUsingDict(src []byte, dict *Dictionary, level int) ([]byte, error) {
	if len(src) == 0 {
		return []byte{}, nil
	}

	if dict == nil || len(dict.dictData) == 0 {
		return z.Compress(src, level)
	}

	// Create a compression context
	cctx := z.createCCtx()
	if cctx == nil {
		return nil, fmt.Errorf("failed to create compression context")
	}
	defer z.freeCCtx(cctx)

	// Create a compression dictionary
	cdict := z.createCDict(
		unsafe.Pointer(&dict.dictData[0]),
		uint64(len(dict.dictData)),
		level,
	)
	if cdict == nil {
		return nil, fmt.Errorf("failed to create compression dictionary")
	}
	defer z.freeCDict(cdict)

	// Allocate output buffer
	dstCapacity := z.compressBound(uint64(len(src)))
	dst := make([]byte, dstCapacity)

	// Compress using dictionary
	result := z.compressUsingCDict(
		cctx,
		unsafe.Pointer(&dst[0]),
		dstCapacity,
		unsafe.Pointer(&src[0]),
		uint64(len(src)),
		cdict,
	)

	// Check for errors
	if z.isError(result) != 0 {
		return nil, fmt.Errorf("dictionary compression error: %s", z.getErrorName(result))
	}

	return dst[:result], nil
}

// DecompressUsingDict decompresses data using the dictionary
func (z *Zstd) DecompressUsingDict(src []byte, dict *Dictionary, maxSize int) ([]byte, error) {
	if len(src) == 0 {
		return []byte{}, nil
	}

	if dict == nil || len(dict.dictData) == 0 {
		return z.Decompress(src, maxSize)
	}

	// If maxSize is 0, use a reasonable default
	if maxSize <= 0 {
		// Use a conservative estimation
		maxSize = len(src) * 5
		if maxSize < 1024 {
			maxSize = 1024 // Minimum reasonable size
		}
	}

	// Create a decompression context
	dctx := z.createDCtx()
	if dctx == nil {
		return nil, fmt.Errorf("failed to create decompression context")
	}
	defer z.freeDCtx(dctx)

	// Create a decompression dictionary
	ddict := z.createDDict(
		unsafe.Pointer(&dict.dictData[0]),
		uint64(len(dict.dictData)),
	)
	if ddict == nil {
		return nil, fmt.Errorf("failed to create decompression dictionary")
	}
	defer z.freeDDict(ddict)

	// Allocate output buffer
	dst := make([]byte, maxSize)

	// Decompress using dictionary
	result := z.decompressUsingDDict(
		dctx,
		unsafe.Pointer(&dst[0]),
		uint64(maxSize),
		unsafe.Pointer(&src[0]),
		uint64(len(src)),
		ddict,
	)

	// Check for errors
	if z.isError(result) != 0 {
		return nil, fmt.Errorf("dictionary decompression error: %s", z.getErrorName(result))
	}

	return dst[:result], nil
}
