package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/RandomCodeSpace/otelcontext/internal/storage"
)

// TestHandleGetLogs_SearchCapRejectsOlderThan24h verifies that an explicit
// search query with a window entirely outside the 24h cap returns 400.
// Symmetric with the MCP search_logs cap so a direct HTTP caller cannot
// bypass via the alternate transport.
func TestHandleGetLogs_SearchCapRejectsOlderThan24h(t *testing.T) {
	repo := newAPITestRepoWithoutFTS(t)
	srv := &Server{repo: repo}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/logs", srv.handleGetLogs)

	q := url.Values{}
	q.Set("search", "panic")
	q.Set("start", time.Now().Add(-5*24*time.Hour).Format(time.RFC3339))
	q.Set("end", time.Now().Add(-4*24*time.Hour).Format(time.RFC3339))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/logs?"+q.Encode(), nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for out-of-cap window, got %d body=%q", rec.Code, rec.Body.String())
	}
}

// TestHandleGetLogs_NoSearchSkipsCap verifies that a filtered listing with
// no search term keeps the full retention range — the cap fires only on
// keyword queries, where unbounded LIKE scans are the worst case.
func TestHandleGetLogs_NoSearchSkipsCap(t *testing.T) {
	repo := newAPITestRepoWithoutFTS(t)
	srv := &Server{repo: repo}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/logs", srv.handleGetLogs)

	q := url.Values{}
	// No search param — listing-only request with a 5-day-old window.
	q.Set("start", time.Now().Add(-5*24*time.Hour).Format(time.RFC3339))
	q.Set("end", time.Now().Add(-4*24*time.Hour).Format(time.RFC3339))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/logs?"+q.Encode(), nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("filtered listing without search must succeed, got %d body=%q", rec.Code, rec.Body.String())
	}
}

// newAPITestRepoWithoutFTS builds a fresh in-memory repo with FTS5 disabled.
// Used by cap tests since they only care about handler behavior, not the
// search backend.
func newAPITestRepoWithoutFTS(t *testing.T) *storage.Repository {
	t.Helper()
	t.Setenv("LOG_FTS_ENABLED", "false")
	db, err := storage.NewDatabase("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	if err := storage.AutoMigrateModels(db, "sqlite"); err != nil {
		t.Fatalf("AutoMigrateModels: %v", err)
	}
	repo := storage.NewRepositoryFromDB(db, "sqlite")
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}
