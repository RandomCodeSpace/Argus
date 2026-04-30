package storage

import (
	"context"
	"testing"
	"time"
)

// TestLogsForVectorReplay_ReturnsErrorAndWarnOnly verifies the severity
// filter matches vectordb.shouldIndex (ERROR/WARN/WARNING/FATAL/CRITICAL).
// INFO and DEBUG rows must be excluded so the page isn't bloated with rows
// vectordb would drop anyway.
func TestLogsForVectorReplay_ReturnsErrorAndWarnOnly(t *testing.T) {
	repo := newTestRepo(t)
	now := time.Now().UTC()
	rows := []Log{
		{TenantID: "default", Severity: "ERROR", Body: "panic", ServiceName: "svc", Timestamp: now},
		{TenantID: "default", Severity: "WARN", Body: "slow", ServiceName: "svc", Timestamp: now},
		{TenantID: "default", Severity: "WARNING", Body: "deprecated", ServiceName: "svc", Timestamp: now},
		{TenantID: "default", Severity: "FATAL", Body: "OOM", ServiceName: "svc", Timestamp: now},
		{TenantID: "default", Severity: "CRITICAL", Body: "deadlock", ServiceName: "svc", Timestamp: now},
		{TenantID: "default", Severity: "INFO", Body: "request handled", ServiceName: "svc", Timestamp: now},
		{TenantID: "default", Severity: "DEBUG", Body: "trace data", ServiceName: "svc", Timestamp: now},
	}
	if err := repo.db.Create(&rows).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := repo.LogsForVectorReplay(context.Background(), 0, 100)
	if err != nil {
		t.Fatalf("LogsForVectorReplay: %v", err)
	}
	if len(got) != 5 {
		t.Errorf("got %d rows, want 5 (ERROR+WARN+WARNING+FATAL+CRITICAL)", len(got))
	}
	for _, l := range got {
		if l.Severity == "INFO" || l.Severity == "DEBUG" {
			t.Errorf("unexpected severity in result: %q (id=%d)", l.Severity, l.ID)
		}
	}
}

// TestLogsForVectorReplay_RespectsSinceID verifies the cursor pagination
// contract: rows with id <= sinceID are excluded so the caller can advance
// across pages without re-fetching.
func TestLogsForVectorReplay_RespectsSinceID(t *testing.T) {
	repo := newTestRepo(t)
	now := time.Now().UTC()
	for range 5 {
		repo.db.Create(&Log{TenantID: "default", Severity: "ERROR", Body: "x", ServiceName: "svc", Timestamp: now})
	}

	page1, err := repo.LogsForVectorReplay(context.Background(), 0, 2)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1: got %d rows, want 2", len(page1))
	}
	// IDs must be strictly ascending.
	if page1[0].ID >= page1[1].ID {
		t.Errorf("page1 not ascending: %d, %d", page1[0].ID, page1[1].ID)
	}

	page2, err := repo.LogsForVectorReplay(context.Background(), page1[1].ID, 2)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("page2: got %d rows, want 2", len(page2))
	}
	for _, r := range page2 {
		if r.ID <= page1[1].ID {
			t.Errorf("page2 contains id=%d <= page1 cursor=%d", r.ID, page1[1].ID)
		}
	}

	page3, err := repo.LogsForVectorReplay(context.Background(), page2[1].ID, 2)
	if err != nil {
		t.Fatalf("page3: %v", err)
	}
	if len(page3) != 1 {
		t.Errorf("page3: got %d rows, want 1 (final partial page)", len(page3))
	}
}

// TestLogsForVectorReplay_CrossTenant verifies the replay is intentionally
// cross-tenant — vectordb is a global accelerator and per-doc tenant tags
// enforce isolation at Search time.
func TestLogsForVectorReplay_CrossTenant(t *testing.T) {
	repo := newTestRepo(t)
	now := time.Now().UTC()
	repo.db.Create(&[]Log{
		{TenantID: "acme", Severity: "ERROR", Body: "a", ServiceName: "svc", Timestamp: now},
		{TenantID: "globex", Severity: "ERROR", Body: "b", ServiceName: "svc", Timestamp: now},
		{TenantID: "default", Severity: "ERROR", Body: "c", ServiceName: "svc", Timestamp: now},
	})

	// No tenant context — replay is cross-tenant by design.
	got, err := repo.LogsForVectorReplay(context.Background(), 0, 100)
	if err != nil {
		t.Fatalf("LogsForVectorReplay: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d rows across tenants, want 3", len(got))
	}
	tenants := map[string]int{}
	for _, l := range got {
		tenants[l.TenantID]++
	}
	for _, name := range []string{"acme", "globex", "default"} {
		if tenants[name] != 1 {
			t.Errorf("tenant %q: got %d rows, want 1", name, tenants[name])
		}
	}
}

// TestLogsForVectorReplay_LimitClamp verifies the limit is clamped to a
// safe default when caller passes 0 / negative / absurdly large values.
func TestLogsForVectorReplay_LimitClamp(t *testing.T) {
	repo := newTestRepo(t)
	now := time.Now().UTC()
	for range 3 {
		repo.db.Create(&Log{TenantID: "default", Severity: "ERROR", Body: "x", ServiceName: "svc", Timestamp: now})
	}

	for _, lim := range []int{0, -1, 999_999} {
		got, err := repo.LogsForVectorReplay(context.Background(), 0, lim)
		if err != nil {
			t.Errorf("limit=%d: unexpected err=%v", lim, err)
			continue
		}
		// 3 rows seeded; default cap is 10k, so all 3 must come back.
		if len(got) != 3 {
			t.Errorf("limit=%d: got %d rows, want 3", lim, len(got))
		}
	}
}
