package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/develerltd/zstd-purego"
)

func main() {
	fmt.Println("Zstd-purego Example")

	// Sample data
	data := []byte("This is a sample string to compress using Zstandard via purego!")
	fmt.Printf("Original size: %d bytes\n", len(data))

	// Basic compression
	fmt.Println("\n=== Basic Compression ===")
	for _, level := range []int{zstd.BestSpeed, zstd.DefaultCompression, zstd.BestCompression} {
		compressed, err := zstd.CompressLevel(data, level)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Compression error at level %d: %v\n", level, err)
			continue
		}

		fmt.Printf("Level %2d: %d bytes (%.2f%% of original)\n",
			level, len(compressed), float64(len(compressed))*100/float64(len(data)))

		// Decompress and verify
		decompressed, err := zstd.Decompress(compressed, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Decompression error: %v\n", err)
			continue
		}

		if !bytes.Equal(data, decompressed) {
			fmt.Fprintf(os.Stderr, "Data mismatch after decompression!\n")
		}
	}

	// Streaming compression
	fmt.Println("\n=== Streaming API ===")
	var compressedBuf bytes.Buffer

	writer, err := zstd.NewWriter(&compressedBuf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create writer: %v\n", err)
		return
	}

	_, err = writer.Write(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write: %v\n", err)
		return
	}

	// Close to flush all data
	err = writer.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to close writer: %v\n", err)
		return
	}

	fmt.Printf("Streaming compressed size: %d bytes\n", compressedBuf.Len())

	// Streaming decompression
	reader, err := zstd.NewReader(&compressedBuf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create reader: %v\n", err)
		return
	}

	decompressed, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read: %v\n", err)
		return
	}

	if !bytes.Equal(data, decompressed) {
		fmt.Fprintf(os.Stderr, "Data mismatch after streaming decompression!\n")
	} else {
		fmt.Println("Streaming decompression successful!")
	}

	// Load Zstd library and get version info
	z, err := zstd.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load Zstd library: %v\n", err)
		return
	}
	defer z.Close()

	fmt.Printf("\nZstandard version: %s (number: %d)\n",
		z.VersionString(), z.Version())
}
