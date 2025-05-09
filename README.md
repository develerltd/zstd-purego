# zstd-purego

[![Go Reference](https://pkg.go.dev/badge/github.com/develerltd/zstd-purego.svg)](https://pkg.go.dev/github.com/develerltd/zstd-purego)

A pure Go binding for the Zstandard (zstd) compression library that doesn't use CGo, based on [purego](https://github.com/ebitengine/purego).

## Features

- No CGo dependency - significantly simplifies cross-compilation
- Embedded shared libraries for supported platforms (Linux amd64, macOS arm64)
- Full API support: simple, context, and streaming operations
- Dictionary compression/decompression support
- Memory-safe wrapper around the C API

## Installation

```bash
go get github.com/develerltd/zstd-purego

## Supported Platforms
- Linux amd64 (with glibc 2.17+)
- macOS arm64 (Apple Silicon)

## Basic Usage

```
package main

import (
	"fmt"

	"github.com/develerltd/zstd-purego"
)

func main() {
	// Compress data with default compression level
	data := []byte("Hello, zstd!")
	compressed, err := zstd.Compress(data)
	if err != nil {
		panic(err)
	}
	
	fmt.Printf("Compressed: %d bytes\n", len(compressed))
	
	// Decompress
	decompressed, err := zstd.Decompress(compressed, 0)
	if err != nil {
		panic(err)
	}
	
	fmt.Printf("Decompressed: %s\n", decompressed)
}
```

## Streaming API

```
// Compressing
var compressedBuf bytes.Buffer
writer, _ := zstd.NewWriter(&compressedBuf)
writer.Write([]byte("Data to compress"))
writer.Close() // Important to flush any remaining data

// Decompressing
reader, _ := zstd.NewReader(&compressedBuf)
decompressed, _ := io.ReadAll(reader)
reader.Close()
```

## Dictionary Compression

```
// Load a pre-trained dictionary
z, _ := zstd.New()
defer z.Close()

dict, _ := z.LoadDictionary(dictData)
compressed, _ := z.CompressUsingDict(data, dict, zstd.DefaultCompression)
decompressed, _ := z.DecompressUsingDict(compressed, dict, 0)
```

## Advanced Usage

```
// Create a custom instance of the library
z, err := zstd.New()
if err != nil {
	panic(err)
}
defer z.Close() // Important to free resources

// Get library version
fmt.Printf("Zstandard version: %s\n", z.VersionString())

// Use best compression
compressed, err := z.Compress(data, zstd.BestCompression)
if err != nil {
	panic(err)
}

// Estimate decompressed size
bound := z.CompressBound(len(data))
fmt.Printf("Maximum compressed size: %d bytes\n", bound)
```

## License
This project is licensed under the MIT License - see the LICENSE file for details.
The Zstandard library is licensed under a dual BSD/GPLv2 license. For more information, see the Zstandard repository.

## Additional Notes About Library Files

For the library to work, you need to include the actual shared libraries in the `libs` directory:

1. For Linux amd64: `libs/linux_amd64_glibc2.17/libzstd.so.1`
2. For macOS arm64: `libs/darwin_arm64/libzstd.dylib`

You can obtain these libraries as mentioned earlier:

### For macOS arm64

On an Apple Silicon Mac:
```bash
# Using Homebrew
brew install zstd
mkdir -p libs/darwin_arm64
cp /opt/homebrew/lib/libzstd.dylib libs/darwin_arm64/
```

## Refresh library linux:

```
docker run --rm -v $(pwd):/work -w /work ubuntu:16.04 /bin/bash -c '
    apt-get update && apt-get install -y curl build-essential cmake
    curl -L https://github.com/facebook/zstd/archive/refs/tags/v1.5.5.tar.gz -o zstd.tar.gz
    tar -xzf zstd.tar.gz
    cd zstd-1.5.5
    make
    mkdir -p /work/libs/linux_amd64_glibc2.17
    cp lib/libzstd.so.1 /work/libs/linux_amd64_glibc2.17/
```

## Refresh on mac:

```
# 1. Install build tools if you haven't already
xcode-select --install
brew install cmake  # Optional but useful

# 2. Download and extract the zstd source
curl -L https://github.com/facebook/zstd/archive/refs/tags/v1.5.5.tar.gz -o zstd.tar.gz
tar -xzf zstd.tar.gz
cd zstd-1.5.5

# 3. Build the library
make

# 4. Copy the library to the project directory
mkdir -p ../libs/darwin_arm64
cp lib/libzstd.1.dylib ../libs/darwin_arm64/libzstd.dylib

# 5. Clean up (optional)
cd ..
rm -rf zstd-1.5.5 zstd.tar.gz
```