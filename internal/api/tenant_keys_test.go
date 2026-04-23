package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/RandomCodeSpace/otelcontext/internal/storage"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "keys.txt")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp keys: %v", err)
	}
	return p
}

func TestTenantKeysFile_LoadAndMatch(t *testing.T) {
	path := writeTempFile(t, `# comment
key-one=acme
key-two: beta

# trailing blank line allowed
`)
	m, err := LoadTenantKeys(path)
	if err != nil {
		t.Fatalf("LoadTenantKeys: %v", err)
	}
	if len(m) != 2 {
		t.Fatalf("want 2 entries, got %d: %+v", len(m), m)
	}
	if m["key-one"] != "acme" {
		t.Errorf("key-one => %q, want acme", m["key-one"])
	}
	if m["key-two"] != "beta" {
		t.Errorf("key-two => %q, want beta", m["key-two"])
	}

	auth := NewTenantKeyAuth(m)
	if !auth.Enabled() {
		t.Fatal("auth should be enabled")
	}
	if tenant, ok := auth.Lookup("key-one"); !ok || tenant != "acme" {
		t.Errorf("Lookup(key-one) = (%q,%v), want (acme,true)", tenant, ok)
	}
	if _, ok := auth.Lookup("nope"); ok {
		t.Error("Lookup(nope) should be false")
	}
}

func TestTenantKeysFile_OverridesHeader(t *testing.T) {
	// Key "admin" is bound to tenant acme. Caller also sets X-Tenant-ID=victim
	// (attacker attempting cross-tenant read). The auth layer must pin ctx to
	// acme regardless of the header.
	path := writeTempFile(t, "admin=acme\n")
	m, err := LoadTenantKeys(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	auth := NewTenantKeyAuth(m)

	var gotTenant string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = storage.TenantFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	h := auth.Middleware("/mcp", inner)

	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("X-Tenant-ID", "victim") // should be OVERRIDDEN
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if gotTenant != "acme" {
		t.Errorf("tenant not pinned from key: got %q, want acme", gotTenant)
	}
}

func TestTenantKeysFile_UnknownKey_401(t *testing.T) {
	path := writeTempFile(t, "admin=acme\n")
	m, err := LoadTenantKeys(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	auth := NewTenantKeyAuth(m)

	h := auth.Middleware("/mcp", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Unknown bearer → 401.
	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	req.Header.Set("Authorization", "Bearer nope")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown key: want 401, got %d", rec.Code)
	}

	// No Authorization header → 401.
	req = httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth: want 401, got %d", rec.Code)
	}

	// Public path must still pass without auth.
	req = httptest.NewRequest(http.MethodGet, "/live", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("public path: want 200, got %d", rec.Code)
	}
}

func TestTenantKeysFile_EmptyPathDisabled(t *testing.T) {
	m, err := LoadTenantKeys("")
	if err != nil {
		t.Fatalf("empty path: %v", err)
	}
	if m != nil {
		t.Errorf("empty path should return nil map, got %+v", m)
	}
	auth := NewTenantKeyAuth(nil)
	if auth.Enabled() {
		t.Error("disabled auth should report Enabled() == false")
	}
}

func TestTenantKeysFile_Malformed(t *testing.T) {
	cases := []string{
		"keyonly\n",
		"=emptykey\n",
		"k=\n",
		"k=t\nk=t2\n", // duplicate
	}
	for i, c := range cases {
		path := writeTempFile(t, c)
		if _, err := LoadTenantKeys(path); err == nil {
			t.Errorf("case %d (%q): expected error", i, c)
		}
	}
}
