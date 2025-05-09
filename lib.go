package zstd

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Embed the Zstandard shared libraries for supported platforms
//
//go:embed libs/linux_amd64_glibc2.17/libzstd.so.1
//go:embed libs/darwin_arm64/libzstd.dylib
var embeddedLibs embed.FS

// Zstd represents an instance of the Zstandard library.
type Zstd struct {
	handle      uintptr
	tempLibPath string

	// Basic functions
	versionNumber func() uint32
	versionString func() string
	compressBound func(srcSize uint64) uint64
	isError       func(code uint64) int
	getErrorName  func(code uint64) string

	// Simple API functions
	compress   func(dst unsafe.Pointer, dstCapacity uint64, src unsafe.Pointer, srcSize uint64, compressionLevel int) uint64
	decompress func(dst unsafe.Pointer, dstCapacity uint64, src unsafe.Pointer, compressedSize uint64) uint64

	// Context API functions
	createCCtx     func() unsafe.Pointer
	freeCCtx       func(ctx unsafe.Pointer) uint64
	compressCCtx   func(ctx unsafe.Pointer, dst unsafe.Pointer, dstCapacity uint64, src unsafe.Pointer, srcSize uint64, compressionLevel int) uint64
	createDCtx     func() unsafe.Pointer
	freeDCtx       func(ctx unsafe.Pointer) uint64
	decompressDCtx func(ctx unsafe.Pointer, dst unsafe.Pointer, dstCapacity uint64, src unsafe.Pointer, srcSize uint64) uint64

	// Stream API functions
	createCStream    func() unsafe.Pointer
	freeCStream      func(zcs unsafe.Pointer) uint64
	compressStream2  func(zcs unsafe.Pointer, output *ZstdOutBuffer, input *ZstdInBuffer, endOp int) uint64
	createDStream    func() unsafe.Pointer
	freeDStream      func(zds unsafe.Pointer) uint64
	decompressStream func(zds unsafe.Pointer, output *ZstdOutBuffer, input *ZstdInBuffer) uint64

	// dictionary functions
	createCDict          func(dictBuffer unsafe.Pointer, dictSize uint64, compressionLevel int) unsafe.Pointer
	freeCDict            func(cdict unsafe.Pointer) uint64
	createDDict          func(dictBuffer unsafe.Pointer, dictSize uint64) unsafe.Pointer
	freeDDict            func(ddict unsafe.Pointer) uint64
	compressUsingCDict   func(ctx unsafe.Pointer, dst unsafe.Pointer, dstCapacity uint64, src unsafe.Pointer, srcSize uint64, cdict unsafe.Pointer) uint64
	decompressUsingDDict func(ctx unsafe.Pointer, dst unsafe.Pointer, dstCapacity uint64, src unsafe.Pointer, srcSize uint64, ddict unsafe.Pointer) uint64
	getDictID            func(dict unsafe.Pointer, dictSize uint64) uint32
}

// ZstdOutBuffer represents a buffer for zstd output operations
type ZstdOutBuffer struct {
	Dst  unsafe.Pointer
	Size uint64
	Pos  uint64
}

// ZstdInBuffer represents a buffer for zstd input operations
type ZstdInBuffer struct {
	Src  unsafe.Pointer
	Size uint64
	Pos  uint64
}

// loadLibrary loads the appropriate Zstd shared library for the current platform
func loadLibrary() (*Zstd, error) {
	tempDir, handle, err := extractAndLoadLibrary()
	if err != nil {
		return nil, err
	}

	z := &Zstd{
		handle:      handle,
		tempLibPath: tempDir,
	}

	// Register basic functions
	purego.RegisterLibFunc(&z.versionNumber, handle, "ZSTD_versionNumber")
	purego.RegisterLibFunc(&z.versionString, handle, "ZSTD_versionString")
	purego.RegisterLibFunc(&z.compressBound, handle, "ZSTD_compressBound")
	purego.RegisterLibFunc(&z.isError, handle, "ZSTD_isError")
	purego.RegisterLibFunc(&z.getErrorName, handle, "ZSTD_getErrorName")

	// Register Simple API functions
	purego.RegisterLibFunc(&z.compress, handle, "ZSTD_compress")
	purego.RegisterLibFunc(&z.decompress, handle, "ZSTD_decompress")

	// Register Context API functions
	purego.RegisterLibFunc(&z.createCCtx, handle, "ZSTD_createCCtx")
	purego.RegisterLibFunc(&z.freeCCtx, handle, "ZSTD_freeCCtx")
	purego.RegisterLibFunc(&z.compressCCtx, handle, "ZSTD_compressCCtx")
	purego.RegisterLibFunc(&z.createDCtx, handle, "ZSTD_createDCtx")
	purego.RegisterLibFunc(&z.freeDCtx, handle, "ZSTD_freeDCtx")
	purego.RegisterLibFunc(&z.decompressDCtx, handle, "ZSTD_decompressDCtx")

	// Register Stream API functions
	purego.RegisterLibFunc(&z.createCStream, handle, "ZSTD_createCStream")
	purego.RegisterLibFunc(&z.freeCStream, handle, "ZSTD_freeCStream")
	purego.RegisterLibFunc(&z.compressStream2, handle, "ZSTD_compressStream2")
	purego.RegisterLibFunc(&z.createDStream, handle, "ZSTD_createDStream")
	purego.RegisterLibFunc(&z.freeDStream, handle, "ZSTD_freeDStream")
	purego.RegisterLibFunc(&z.decompressStream, handle, "ZSTD_decompressStream")

	return z, nil
}

// extractAndLoadLibrary extracts the embedded library for the current platform and loads it
func extractAndLoadLibrary() (string, uintptr, error) {
	// Determine which library to use based on the platform
	var libPath string
	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "amd64" {
			libPath = "libs/linux_amd64_glibc2.17/libzstd.so.1"
		} else {
			return "", 0, fmt.Errorf("unsupported Linux architecture: %s", runtime.GOARCH)
		}
	case "darwin":
		if runtime.GOARCH == "arm64" {
			libPath = "libs/darwin_arm64/libzstd.dylib"
		} else {
			return "", 0, fmt.Errorf("unsupported macOS architecture: %s", runtime.GOARCH)
		}
	default:
		return "", 0, fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Create a temporary directory to extract the library
	tempDir, err := os.MkdirTemp("", "zstd-lib")
	if err != nil {
		return "", 0, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Extract the library file
	libFile, err := embeddedLibs.Open(libPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", 0, fmt.Errorf("failed to open embedded library: %w", err)
	}
	defer libFile.Close()

	// Create a temporary file for the library
	_, libFilename := filepath.Split(libPath)
	tempLibPath := filepath.Join(tempDir, libFilename)
	outFile, err := os.Create(tempLibPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", 0, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Copy the library content
	_, err = io.Copy(outFile, libFile)
	outFile.Close()
	if err != nil {
		os.RemoveAll(tempDir)
		return "", 0, fmt.Errorf("failed to write temp library file: %w", err)
	}

	// Set execution permissions for the library
	if runtime.GOOS != "windows" {
		err = os.Chmod(tempLibPath, 0755) // rwxr-xr-x
		if err != nil {
			os.RemoveAll(tempDir)
			return "", 0, fmt.Errorf("failed to set library permissions: %w", err)
		}
	}

	// Load the library using purego
	handle, err := purego.Dlopen(tempLibPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		os.RemoveAll(tempDir)
		return "", 0, fmt.Errorf("failed to load library: %w", err)
	}

	return tempDir, handle, nil
}

// closeLibrary releases the shared library and cleans up temporary files
func (z *Zstd) closeLibrary() error {
	var err error
	if z.handle != 0 {
		err = purego.Dlclose(z.handle)
	}

	// Clean up the temporary directory
	if z.tempLibPath != "" {
		os.RemoveAll(z.tempLibPath)
	}

	return err
}
