package api

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/RandomCodeSpace/otelcontext/internal/storage"
)

// LoadTenantKeys parses a tenant-keys file where each non-empty,
// non-comment line is a `key=tenant` pair. The returned map is
// keyed by bearer token (value=tenant ID) so an authenticated
// request can be scoped to the key's bound tenant regardless of
// any client-asserted X-Tenant-ID header.
//
// File format (YAML/INI-friendly, also accepts plain `key=tenant`):
//
//	# one mapping per line, comments start with '#'
//	5f4dcc3b5aa765d61d8327deb882cf99=acme
//	c20ad4d76fe97759aa27a0c99bff6710=beta
//
// Whitespace around tokens is ignored; duplicate keys return an error
// so misconfiguration fails loud at startup.
func LoadTenantKeys(path string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path) // #nosec G304 -- operator-supplied config path
	if err != nil {
		return nil, fmt.Errorf("tenant keys file %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	out := make(map[string]string)
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		// Accept both `key=tenant` and `key: tenant` (YAML-ish).
		sep := "="
		if !strings.Contains(raw, "=") && strings.Contains(raw, ":") {
			sep = ":"
		}
		parts := strings.SplitN(raw, sep, 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("tenant keys file %q line %d: expected `key=tenant`, got %q", path, line, raw)
		}
		key := strings.TrimSpace(parts[0])
		tenant := strings.TrimSpace(parts[1])
		if key == "" || tenant == "" {
			return nil, fmt.Errorf("tenant keys file %q line %d: empty key or tenant", path, line)
		}
		if _, dup := out[key]; dup {
			return nil, fmt.Errorf("tenant keys file %q line %d: duplicate key", path, line)
		}
		out[key] = tenant
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("tenant keys file %q: %w", path, err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("tenant keys file %q: no entries found", path)
	}
	return out, nil
}

// TenantKeyAuth holds a key→tenant map loaded from APITenantKeysFile and
// exposes middleware that both authenticates AND pins the tenant onto the
// request context. When enabled it OVERRIDES any X-Tenant-ID header — callers
// cannot cross tenants by swapping headers.
type TenantKeyAuth struct {
	mu      sync.RWMutex
	entries map[string]string // key → tenant
}

// NewTenantKeyAuth constructs a TenantKeyAuth from a pre-loaded map.
// nil or empty disables the layer; callers should fall back to the shared
// API_KEY path in that case.
func NewTenantKeyAuth(entries map[string]string) *TenantKeyAuth {
	cp := make(map[string]string, len(entries))
	for k, v := range entries {
		cp[k] = v
	}
	return &TenantKeyAuth{entries: cp}
}

// Enabled reports whether per-tenant keys are configured.
func (a *TenantKeyAuth) Enabled() bool {
	if a == nil {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.entries) > 0
}

// Lookup returns the tenant bound to the given bearer key, using a
// constant-time compare against every entry so mismatches don't leak via
// timing.
func (a *TenantKeyAuth) Lookup(key string) (string, bool) {
	if a == nil || key == "" {
		return "", false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	got := []byte(key)
	var matchedTenant string
	var matched int
	for k, tenant := range a.entries {
		// subtle.ConstantTimeCompare requires equal length; guard before call.
		if len(got) == len(k) && subtle.ConstantTimeCompare(got, []byte(k)) == 1 {
			matchedTenant = tenant
			matched = 1
		}
	}
	if matched == 1 {
		return matchedTenant, true
	}
	return "", false
}

// Middleware returns an http.Handler wrapper that requires an
// `Authorization: Bearer <key>` header present in the tenant-keys map.
// On success the matched tenant is pinned onto the request context via
// storage.WithTenantContext, overriding any client-supplied X-Tenant-ID.
// On mismatch (including missing header) it returns 401.
//
// Public paths (IsProtectedPath == false) pass through unchanged so UI
// assets and health probes remain reachable without credentials.
func (a *TenantKeyAuth) Middleware(mcpPath string, next http.Handler) http.Handler {
	if !a.Enabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsProtectedPath(r.URL.Path, mcpPath) {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(auth, prefix) {
			writeUnauthorized(w)
			return
		}
		got := strings.TrimPrefix(auth, prefix)
		tenant, ok := a.Lookup(got)
		if !ok {
			writeUnauthorized(w)
			return
		}
		// Pin tenant onto ctx. This OVERRIDES any X-Tenant-ID header a
		// caller may have set, closing the cross-tenant read vector.
		ctx := storage.WithTenantContext(r.Context(), tenant)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
