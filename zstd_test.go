package zstd

import (
	"bytes"
	"testing"
)

func TestBasicCompressDecompress(t *testing.T) {
	original := []byte("Hello, zstd-purego!")

	// Compress
	compressed, err := Compress(original)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}

	// Verify compression actually happened
	if len(compressed) >= len(original) {
		t.Logf("Warning: Compressed size not smaller than original for small test data")
	}

	// Decompress
	decompressed, err := Decompress(compressed, 0)
	if err != nil {
		t.Fatalf("Decompression failed: %v", err)
	}

	// Verify result
	if !bytes.Equal(original, decompressed) {
		t.Errorf("Decompressed data doesn't match original")
	}
}

func TestLibraryVersion(t *testing.T) {
	z, err := New()
	if err != nil {
		t.Fatalf("Failed to load library: %v", err)
	}
	defer z.Close()

	version := z.VersionString()
	if version == "" {
		t.Errorf("Library version should not be empty")
	}
	t.Logf("Loaded zstd library version: %s", version)
}
