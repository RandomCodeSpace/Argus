package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AuthFailureHook is an optional callback invoked whenever API-key auth rejects
// a request. Set by main.go to increment the APIAuthFailuresTotal metric.
// Left as a package-level function pointer (rather than a DI parameter) to
// avoid circular imports between api and telemetry. Safe to leave nil.
//
// Reasons: "missing_header", "bad_scheme", "bad_key".
var AuthFailureHook func(reason string)

func recordAuthFailure(reason string) {
	if AuthFailureHook != nil {
		AuthFailureHook(reason)
	}
}

// RequireAPIKey returns middleware that requires an `Authorization: Bearer <key>`
// header matching the configured API key. When expectedKey is empty the
// middleware is a pass-through (auth disabled) — the caller is expected to
// log a warning at startup in that case.
//
// The comparison is constant-time via subtle.ConstantTimeCompare to avoid
// timing side channels. On mismatch or missing header a 401 is returned with
// a JSON body `{"error":"unauthorized"}`.
func RequireAPIKey(expectedKey string, next http.Handler) http.Handler {
	if expectedKey == "" {
		return next
	}
	expected := []byte(expectedKey)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if auth == "" {
			recordAuthFailure("missing_header")
			writeUnauthorized(w)
			return
		}
		if !strings.HasPrefix(auth, prefix) {
			recordAuthFailure("bad_scheme")
			writeUnauthorized(w)
			return
		}
		got := []byte(strings.TrimPrefix(auth, prefix))
		if subtle.ConstantTimeCompare(got, expected) != 1 {
			recordAuthFailure("bad_key")
			writeUnauthorized(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}

// IsProtectedPath reports whether a request path requires API-key authentication.
// Protected: /api/*, /v1/* (OTLP HTTP), and the MCP path.
// Unprotected: /live, /ready, /health*, /metrics* (Prometheus), /ws* (WebSocket),
// and the UI static bundle ("/" + assets).
func IsProtectedPath(path, mcpPath string) bool {
	// Explicit skip-list for health/metrics/ws endpoints that may live under /api.
	switch {
	case path == "/live", path == "/ready":
		return false
	case strings.HasPrefix(path, "/health"):
		return false
	case strings.HasPrefix(path, "/metrics"):
		return false
	case strings.HasPrefix(path, "/ws"):
		return false
	case path == "/api/health":
		return false
	}
	if strings.HasPrefix(path, "/api/") {
		return true
	}
	if strings.HasPrefix(path, "/v1/") {
		return true
	}
	if mcpPath != "" && (path == mcpPath || strings.HasPrefix(path, mcpPath+"/")) {
		return true
	}
	return false
}

// APIKeyGate wraps a handler so only requests matching IsProtectedPath require the key.
// Public paths flow through untouched, which is what keeps the UI bundle and health
// probes accessible without credentials.
func APIKeyGate(expectedKey, mcpPath string, next http.Handler) http.Handler {
	if expectedKey == "" {
		return next
	}
	protected := RequireAPIKey(expectedKey, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IsProtectedPath(r.URL.Path, mcpPath) {
			protected.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
