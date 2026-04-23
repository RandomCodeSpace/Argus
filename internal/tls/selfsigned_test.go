package tlsbootstrap

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func parseCert(t *testing.T, path string) *x509.Certificate {
	t.Helper()
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatal("pem.Decode returned nil")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	return cert
}

func TestEnsureSelfSignedCert_CreatesNew(t *testing.T) {
	dir := t.TempDir()

	certPath, keyPath, err := EnsureSelfSignedCert(dir)
	if err != nil {
		t.Fatalf("EnsureSelfSignedCert: %v", err)
	}

	certInfo, err := os.Stat(certPath)
	if err != nil {
		t.Fatalf("stat cert: %v", err)
	}
	keyInfo, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key: %v", err)
	}

	// Mode assertions — key must be private (0600), cert may be world-readable (0644).
	if mode := certInfo.Mode().Perm(); mode != 0o644 {
		t.Errorf("cert mode = %o, want 0644", mode)
	}
	if mode := keyInfo.Mode().Perm(); mode != 0o600 {
		t.Errorf("key mode = %o, want 0600", mode)
	}

	// SAN assertions.
	cert := parseCert(t, certPath)
	var hasLocalhost bool
	for _, d := range cert.DNSNames {
		if d == "localhost" {
			hasLocalhost = true
		}
	}
	if !hasLocalhost {
		t.Errorf("expected DNSNames to include localhost, got %v", cert.DNSNames)
	}
	var has127 bool
	for _, ip := range cert.IPAddresses {
		if ip.Equal(net.IPv4(127, 0, 0, 1)) {
			has127 = true
		}
	}
	if !has127 {
		t.Errorf("expected IPAddresses to include 127.0.0.1, got %v", cert.IPAddresses)
	}
	if cert.Subject.CommonName != "otelcontext" {
		t.Errorf("CommonName = %q, want otelcontext", cert.Subject.CommonName)
	}
	if !cert.IsCA {
		t.Error("expected self-CA cert (IsCA=true)")
	}
}

func TestEnsureSelfSignedCert_ReusesValid(t *testing.T) {
	dir := t.TempDir()

	certPath, _, err := EnsureSelfSignedCert(dir)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	info1, err := os.Stat(certPath)
	if err != nil {
		t.Fatal(err)
	}

	// Force an observable gap so any rewrite would change mtime.
	time.Sleep(20 * time.Millisecond)

	certPath2, _, err := EnsureSelfSignedCert(dir)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if certPath != certPath2 {
		t.Fatalf("paths differ: %q vs %q", certPath, certPath2)
	}
	info2, err := os.Stat(certPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Fatalf("cert was rewritten: mtime %v -> %v", info1.ModTime(), info2.ModTime())
	}
}

func TestEnsureSelfSignedCert_RegeneratesIfExpired(t *testing.T) {
	dir := t.TempDir()

	// Seed an obviously-expired cert/key pair.
	expiredCertPEM := `-----BEGIN CERTIFICATE-----
MIIBUTCB+aADAgECAgEBMAoGCCqGSM49BAMCMBYxFDASBgNVBAMTC290ZWxjb250
ZXh0MB4XDTAwMDEwMTAwMDAwMFoXDTAxMDEwMTAwMDAwMFowFjEUMBIGA1UEAxML
b3RlbGNvbnRleHQwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARxK8MZbN4iGk4l
yUqFJZgQaQl6P8t9C2e0kRpF2/k5bXZJ9oZtMrWq0bq4cEk6Qn8u+5w4tUeH0SgI
ZzPBmFYRoyEwHzAdBgNVHQ4EFgQUKz6iS4q7t9h4rI9oB8IY5fR4tz0wCgYIKoZI
zj0EAwIDSAAwRQIgC/nDrqT/8R/lQyxh/5YIc76b3rE2L6xjMQJcEzRzTe4CIQDy
yE1VmRlW4c2oVp+/cEpH/0fLJoZKJfN3o2Bqv+7s7Q==
-----END CERTIFICATE-----
`
	certPath := filepath.Join(dir, certFileName)
	keyPath := filepath.Join(dir, keyFileName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, []byte(expiredCertPEM), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("placeholder"), 0o600); err != nil {
		t.Fatal(err)
	}

	gotCert, _, err := EnsureSelfSignedCert(dir)
	if err != nil {
		t.Fatalf("EnsureSelfSignedCert: %v", err)
	}
	if gotCert != certPath {
		t.Fatalf("unexpected cert path: %q", gotCert)
	}

	// New cert must be valid well into the future.
	cert := parseCert(t, certPath)
	if cert.NotAfter.Before(time.Now().Add(365 * 24 * time.Hour)) {
		t.Fatalf("expected long-lived replacement cert, got NotAfter=%v", cert.NotAfter)
	}
}

func TestEnsureSelfSignedCert_ConcurrentCallers(t *testing.T) {
	dir := t.TempDir()

	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := EnsureSelfSignedCert(dir); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent caller failed: %v", err)
	}

	// Final cert must still be valid.
	cert := parseCert(t, filepath.Join(dir, certFileName))
	if time.Now().After(cert.NotAfter) {
		t.Fatal("resulting cert is already expired")
	}
}
