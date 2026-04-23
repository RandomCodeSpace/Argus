package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/RandomCodeSpace/otelcontext/internal/telemetry"
)

// DBPinger is the minimum DB interface DBHealth needs. *sql.DB satisfies it;
// tests can pass a stub.
type DBPinger interface {
	PingContext(ctx context.Context) error
}

// DBHealth periodically pings the database and exposes the result via an
// atomic boolean. The HTTP middleware below short-circuits /api/* traffic
// with a 503 when the flag is false, preventing goroutine pile-up on pool
// acquisition when the DB is unreachable.
type DBHealth struct {
	db       DBPinger
	driver   string
	interval time.Duration
	healthy  atomic.Bool
	metrics  *telemetry.Metrics
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewDBHealth constructs a health poller. Default poll interval is 5s; ping
// timeout is 2s per attempt. Start() must be called to begin polling.
func NewDBHealth(db DBPinger, driver string, metrics *telemetry.Metrics) *DBHealth {
	h := &DBHealth{
		db:       db,
		driver:   driver,
		interval: 5 * time.Second,
		metrics:  metrics,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	// Assume healthy until the first ping proves otherwise — avoids a
	// spurious 503 window at startup.
	h.healthy.Store(true)
	if metrics != nil && metrics.DBUp != nil {
		metrics.DBUp.WithLabelValues(driver).Set(1)
	}
	return h
}

// Start launches the background poller.
func (h *DBHealth) Start(ctx context.Context) {
	go h.loop(ctx)
}

// Stop signals the poller to exit and waits briefly for it to finish.
func (h *DBHealth) Stop() {
	select {
	case <-h.stopCh:
		// already stopped
	default:
		close(h.stopCh)
	}
	select {
	case <-h.doneCh:
	case <-time.After(2 * time.Second):
	}
}

// Healthy reports the most recent ping result.
func (h *DBHealth) Healthy() bool { return h.healthy.Load() }

func (h *DBHealth) loop(ctx context.Context) {
	defer close(h.doneCh)
	tick := time.NewTicker(h.interval)
	defer tick.Stop()
	h.ping(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopCh:
			return
		case <-tick.C:
			h.ping(ctx)
		}
	}
}

func (h *DBHealth) ping(parent context.Context) {
	if h.db == nil {
		return
	}
	pctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	err := h.db.PingContext(pctx)
	up := err == nil
	h.healthy.Store(up)
	if h.metrics != nil && h.metrics.DBUp != nil {
		if up {
			h.metrics.DBUp.WithLabelValues(h.driver).Set(1)
		} else {
			h.metrics.DBUp.WithLabelValues(h.driver).Set(0)
		}
	}
}

// dbHealthSkipPath mirrors the auth skip-list: probes, metrics, and the UI
// bundle must stay reachable even when the DB is down so operators can still
// see liveness and scraped metrics.
func dbHealthSkipPath(path string) bool {
	switch {
	case path == "/live", path == "/ready", path == "/health":
		return true
	case strings.HasPrefix(path, "/metrics"):
		return true
	case strings.HasPrefix(path, "/ws"):
		return true
	case path == "/api/health":
		return true
	}
	// Everything else under /api/ or /v1/ is DB-backed.
	if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/v1/") {
		return false
	}
	// UI static bundle and all other paths — pass through.
	return true
}

// DBHealthMiddleware returns 503 immediately when the DB poller reports
// unhealthy, for DB-dependent paths. Health/metrics/UI paths bypass the gate.
func DBHealthMiddleware(h *DBHealth) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if h == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if dbHealthSkipPath(r.URL.Path) || h.Healthy() {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"database unavailable"}`))
		})
	}
}

// Compile-time assertion that *sql.DB satisfies DBPinger.
var _ DBPinger = (*sql.DB)(nil)
