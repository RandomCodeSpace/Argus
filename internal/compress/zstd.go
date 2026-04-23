package compress

import (
	"fmt"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// MaxDecompressedSize caps the output size of a single Decompress call.
// This protects against zstd decompression bombs: small adversarial inputs
// that would otherwise expand to arbitrary sizes and OOM the process.
// 64 MiB is 3-4 orders of magnitude above realistic per-row payload sizes
// (log bodies, attribute JSON are normally KiB-range).
const MaxDecompressedSize = 64 << 20 // 64 MiB

var (
	encoderPool sync.Pool
	decoderPool sync.Pool
)

func init() {
	encoderPool = sync.Pool{
		New: func() any {
			enc, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
			return enc
		},
	}
	decoderPool = sync.Pool{
		New: func() any {
			// WithDecoderMaxMemory caps the decompressed output size. Exceeding
			// the cap causes DecodeAll to return an error rather than allocating
			// unbounded memory.
			dec, _ := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(MaxDecompressedSize))
			return dec
		},
	}
}

// Compress compresses the input data using Zstandard.
func Compress(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	enc := encoderPool.Get().(*zstd.Encoder)
	defer encoderPool.Put(enc)
	return enc.EncodeAll(data, make([]byte, 0, len(data)/2))
}

// Decompress decompresses the input data using Zstandard.
// The output is capped at MaxDecompressedSize; larger outputs return an error.
func Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	dec := decoderPool.Get().(*zstd.Decoder)
	defer decoderPool.Put(dec)
	result, err := dec.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("zstd decompression failed: %w", err)
	}
	// Defense-in-depth: belt-and-braces check in case the library option
	// is bypassed or the cap is raised above available memory.
	if len(result) > MaxDecompressedSize {
		return nil, fmt.Errorf("zstd decompression failed: output %d bytes exceeds max %d", len(result), MaxDecompressedSize)
	}
	return result, nil
}
