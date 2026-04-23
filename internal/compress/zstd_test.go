package compress

import (
	"bytes"
	"testing"
)

// TestDecompress_HappyPath confirms typical payloads round-trip successfully.
func TestDecompress_HappyPath(t *testing.T) {
	sizes := []int{
		1 << 10,  // 1 KiB
		10 << 10, // 10 KiB
		1 << 20,  // 1 MiB
	}
	for _, n := range sizes {
		orig := bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. "), n/45+1)
		orig = orig[:n]

		compressed := Compress(orig)
		if len(compressed) == 0 {
			t.Fatalf("Compress returned empty for %d bytes", n)
		}

		got, err := Decompress(compressed)
		if err != nil {
			t.Fatalf("Decompress returned error for %d bytes: %v", n, err)
		}
		if !bytes.Equal(got, orig) {
			t.Fatalf("round-trip mismatch for %d bytes: len(got)=%d", n, len(got))
		}
	}
}

// TestDecompress_EmptyInput confirms empty input returns nil, no error.
func TestDecompress_EmptyInput(t *testing.T) {
	got, err := Decompress(nil)
	if err != nil {
		t.Fatalf("unexpected error on nil input: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil output, got %d bytes", len(got))
	}
}

// TestDecompress_BombCapped confirms a crafted small zstd stream that would
// expand beyond MaxDecompressedSize is rejected rather than allocated.
//
// Construction: a 70 MiB block of a single repeating byte compresses down
// to a tiny zstd stream (<1 KiB). Without the cap, Decompress would allocate
// the full 70 MiB. With the cap (64 MiB), DecodeAll must error.
//
// This test fails prior to the WithDecoderMaxMemory / size-check fix.
func TestDecompress_BombCapped(t *testing.T) {
	bombSize := 70 << 20 // 70 MiB, intentionally above MaxDecompressedSize (64 MiB)
	payload := bytes.Repeat([]byte{'A'}, bombSize)

	compressed := Compress(payload)
	if len(compressed) == 0 {
		t.Fatalf("Compress returned empty for bomb payload")
	}
	// Sanity: the compressed "bomb" must itself be small — that's the whole
	// point of a decompression bomb. If this ever regresses, the test is
	// no longer exercising the bomb scenario.
	if len(compressed) > 1<<20 {
		t.Fatalf("compressed bomb unexpectedly large: %d bytes", len(compressed))
	}

	got, err := Decompress(compressed)
	if err == nil {
		t.Fatalf("expected error decompressing bomb (%d bytes), got %d bytes output", bombSize, len(got))
	}
	if len(got) != 0 {
		t.Fatalf("expected no output on bomb error, got %d bytes", len(got))
	}
}

// TestDecompress_UnderCap confirms payloads just under the cap still succeed.
func TestDecompress_UnderCap(t *testing.T) {
	size := MaxDecompressedSize - (1 << 20) // 63 MiB
	payload := bytes.Repeat([]byte{'B'}, size)

	compressed := Compress(payload)
	got, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("unexpected error for %d byte payload: %v", size, err)
	}
	if len(got) != size {
		t.Fatalf("size mismatch: want %d, got %d", size, len(got))
	}
}
