package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RandomCodeSpace/otelcontext/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// newTestMetrics constructs a Metrics instance with just the
// PanicsRecoveredTotal counter registered against a local registry, so tests
// can assert on the metric without colliding with the global default
// registry that telemetry.New() uses.
func newTestMetrics(t *testing.T) *telemetry.Metrics {
	t.Helper()
	reg := prometheus.NewRegistry()
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "test_panics_recovered_total",
		Help: "test",
	}, []string{"subsystem"})
	reg.MustRegister(counter)
	return &telemetry.Metrics{PanicsRecoveredTotal: counter}
}

func TestRecoverMiddleware_Http_500(t *testing.T) {
	metrics := newTestMetrics(t)
	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	h := RecoverMiddleware(metrics, panicking)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/whatever", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != `{"error":"internal"}` {
		t.Fatalf("unexpected body: %q", got)
	}

	var m dto.Metric
	if err := metrics.PanicsRecoveredTotal.WithLabelValues("http").Write(&m); err != nil {
		t.Fatalf("metric write: %v", err)
	}
	if got := m.GetCounter().GetValue(); got != 1 {
		t.Fatalf("expected panic counter=1, got %v", got)
	}
}

func TestRecoverMiddleware_PassThrough(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	h := RecoverMiddleware(nil, ok)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected 418 passthrough, got %d", rec.Code)
	}
}
