package ingest

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"testing"

	"google.golang.org/protobuf/proto"
)

// TestOTLPHTTP_PostGzipBombRejected ensures the HTTP OTLP handler rejects a
// gzipped body whose decompressed size exceeds the post-gzip cap, even when
// the compressed wire size is tiny. Without the fix a 1 KiB gzipped body of
// repeated zeros could decompress to hundreds of MiB.
func TestOTLPHTTP_PostGzipBombRejected(t *testing.T) {
	h := newE2EHarness(t)

	// Shrink the decompressed cap so the test is fast and its intent is
	// obvious — 64 MiB+ inputs in a unit test are wasteful.
	h.handler.SetMaxDecompressedBytes(1 * 1024 * 1024) // 1 MiB

	// Build a compressed payload that decompresses to >1 MiB (all zero bytes
	// compress extremely well).
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	zero := make([]byte, 64*1024) // 64 KiB of zeroes
	for i := 0; i < 64; i++ {     // → 4 MiB decompressed, well over the 1 MiB cap
		if _, err := gz.Write(zero); err != nil {
			t.Fatalf("gz.Write: %v", err)
		}
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gz.Close: %v", err)
	}
	if buf.Len() > 4*1024 {
		// Compressed size should be tiny — if it isn't the test is broken.
		t.Logf("compressed size=%d (expected tiny)", buf.Len())
	}

	req, err := http.NewRequest(http.MethodPost, h.server.URL+"/v1/traces", &buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", contentTypeProtobuf)
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413 for gzip bomb, got %d", resp.StatusCode)
	}

	// No row should have landed — verify spanCallback was never called.
	if n := h.spanCalls.Load(); n != 0 {
		t.Fatalf("span callback fired %d times despite 413", n)
	}
}

// TestOTLPHTTP_GzipBelowLimitAccepted confirms the fix is not overly eager:
// a well-formed gzipped request under the cap must still be accepted.
func TestOTLPHTTP_GzipBelowLimitAccepted(t *testing.T) {
	h := newE2EHarness(t)
	h.handler.SetMaxDecompressedBytes(4 * 1024 * 1024)

	// Build a real (tiny) OTLP logs request so the handler actually unmarshals.
	logsReq := buildLogsRequest("gzip-svc", 1)
	data, err := proto.Marshal(logsReq)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		t.Fatalf("gz.Write: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gz.Close: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, h.server.URL+"/v1/logs", &buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", contentTypeProtobuf)
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 for valid gzip body, got %d", resp.StatusCode)
	}
}
