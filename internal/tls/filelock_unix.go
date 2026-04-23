//go:build !windows

package tlsbootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// acquireDirLock obtains an exclusive advisory lock (flock LOCK_EX) on a
// sentinel file in cacheDir. It blocks until the lock is available, so two
// OtelContext processes sharing the same TLS_CACHE_DIR serialise their cert
// generation rather than racing on the cert.pem/key.pem rename.
//
// Returns a release function that drops the lock and closes the sentinel fd;
// callers MUST invoke it (typically via defer) before returning.
func acquireDirLock(cacheDir string) (func(), error) {
	lockPath := filepath.Join(cacheDir, "tls.lock")
	// 0o600 — lock file lives in the same private dir as the key material.
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600) // #nosec G304 -- path joined under operator-controlled cacheDir
	if err != nil {
		return nil, fmt.Errorf("tlsbootstrap: open lock file %q: %w", lockPath, err)
	}
	fd := int(f.Fd()) // #nosec G115 -- os.File.Fd returns uintptr < math.MaxInt on all supported platforms
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("tlsbootstrap: flock %q: %w", lockPath, err)
	}
	release := func() {
		_ = syscall.Flock(fd, syscall.LOCK_UN)
		_ = f.Close()
	}
	return release, nil
}
