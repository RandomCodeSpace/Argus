package tlsbootstrap

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestEnsureSelfSignedCert_CrossProcessSerialization exercises the file-lock
// path within a single process: N goroutines race to generate into the same
// cacheDir and must all return a valid, mutually consistent cert. Because the
// package-level mutex serialises goroutines before they reach the lock, the
// lock itself only gets a real workout when separate OS processes share the
// dir — we cannot cheaply spawn a sibling process from `go test`, but we can
// at least prove that the locked path does not break functionality and all
// goroutines observe the same final cert.
func TestEnsureSelfSignedCert_CrossProcessSerialization(t *testing.T) {
	dir := t.TempDir()

	const n = 16
	var wg sync.WaitGroup
	paths := make([]string, n)
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cp, _, err := EnsureSelfSignedCert(dir)
			paths[idx] = cp
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	first := paths[0]
	for i, p := range paths {
		if errs[i] != nil {
			t.Errorf("goroutine %d: %v", i, errs[i])
		}
		if p != first {
			t.Errorf("goroutine %d: cert path %q != first %q", i, p, first)
		}
	}

	// The resulting cert must still parse and be unexpired.
	cert := parseCert(t, filepath.Join(dir, certFileName))
	if time.Now().After(cert.NotAfter) {
		t.Fatal("resulting cert is already expired")
	}

	// The lock file must have been cleaned up OR still exist but be safe to
	// re-acquire; either way a subsequent call must succeed.
	if _, _, err := EnsureSelfSignedCert(dir); err != nil {
		t.Fatalf("post-concurrent call failed: %v", err)
	}
}
