//go:build windows

package tlsbootstrap

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// acquireDirLock obtains a cross-process sentinel lock on Windows using
// O_CREATE|O_EXCL with bounded retry. Windows lacks flock, but atomically
// creating a lockfile that the releaser deletes gives us mutual exclusion at
// the granularity we need (cert generation, which happens at most once per
// 10-year window).
func acquireDirLock(cacheDir string) (func(), error) {
	lockPath := filepath.Join(cacheDir, "tls.lock")
	const (
		maxAttempts = 300 // ~30s at 100ms
		pollDelay   = 100 * time.Millisecond
	)
	for i := 0; i < maxAttempts; i++ {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) // #nosec G304 -- path joined under operator-controlled cacheDir
		if err == nil {
			_ = f.Close()
			release := func() { _ = os.Remove(lockPath) }
			return release, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("tlsbootstrap: open lock file %q: %w", lockPath, err)
		}
		time.Sleep(pollDelay)
	}
	return nil, fmt.Errorf("tlsbootstrap: timeout acquiring lock %q", lockPath)
}
