package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type stubPinger struct {
	fail atomic.Bool
}

func (s *stubPinger) PingContext(_ context.Context) error {
	if s.fail.Load() {
		return errors.New("down")
	}
	return nil
}

func TestDBHealth_TogglesOnPingFailure(t *testing.T) {
	p := &stubPinger{}
	// Pass nil for metrics — DBHealth guards each metric access with a nil
	// check, and the global promauto registry would collide across tests.
	h := NewDBHealth(p, "sqlite", nil)
	h.interval = 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.Start(ctx)
	defer h.Stop()

	if !h.Healthy() {
		t.Fatalf("expected initial healthy=true")
	}

	p.fail.Store(true)
	waitFor(t, 500*time.Millisecond, func() bool { return !h.Healthy() })

	mw := DBHealthMiddleware(h)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on /api/logs when unhealthy, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/live", nil)
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected /live passthrough, got %d", rec.Code)
	}

	p.fail.Store(false)
	waitFor(t, 500*time.Millisecond, func() bool { return h.Healthy() })
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after recovery, got %d", rec.Code)
	}
}

func waitFor(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v", d)
}
