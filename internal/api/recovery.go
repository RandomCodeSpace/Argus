package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/RandomCodeSpace/otelcontext/internal/telemetry"
)

// RecoverMiddleware catches panics from downstream handlers/middleware, logs
// the stack trace, increments the panics-recovered metric, and responds with
// a generic 500. It must be installed as the OUTERMOST middleware (after
// MetricsMiddleware is wrapped around the stack) so panics anywhere below
// are caught. http.ErrAbortHandler is re-panicked to preserve net/http's
// sentinel-abort contract.
func RecoverMiddleware(metrics *telemetry.Metrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			// Quote the path/method so any embedded newlines or log-forging
			// bytes from attacker-controlled input are rendered as escape
			// sequences rather than break the log line.
			slog.Error("panic recovered", //nolint:gosec // G706 false positive: values are %q-quoted below.
				"path", fmt.Sprintf("%q", r.URL.Path),
				"method", fmt.Sprintf("%q", r.Method),
				"panic", rec,
				"stack", string(debug.Stack()),
			)
			if metrics != nil && metrics.PanicsRecoveredTotal != nil {
				metrics.PanicsRecoveredTotal.WithLabelValues("http").Inc()
			}
			// Best-effort 500; if headers already flushed WriteHeader is a no-op.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal"}`))
		}()
		next.ServeHTTP(w, r)
	})
}
