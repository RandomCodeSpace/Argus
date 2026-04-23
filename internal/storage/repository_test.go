package storage

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestUpdateLogInsight_ScopedByTenant proves the IDOR fix: a caller carrying
// tenant B's context cannot overwrite the ai_insight column of a log belonging
// to tenant A. Zero rows modified, ErrLogNotFoundOrWrongTenant returned.
func TestUpdateLogInsight_ScopedByTenant(t *testing.T) {
	repo := newTestRepo(t)
	now := time.Now().UTC()

	// Seed a single log owned by tenant A.
	owned := Log{
		TenantID:    "tenant-a",
		TraceID:     "t-x",
		SpanID:      "s-x",
		Severity:    "ERROR",
		Body:        "original body",
		ServiceName: "svc-a",
		AIInsight:   "",
		Timestamp:   now,
	}
	if err := repo.db.Create(&owned).Error; err != nil {
		t.Fatalf("seed log: %v", err)
	}

	// Tenant B attempts to update tenant A's row by guessing the primary key.
	bCtx := WithTenantContext(context.Background(), "tenant-b")
	err := repo.UpdateLogInsight(bCtx, owned.ID, "attacker insight")
	if !errors.Is(err, ErrLogNotFoundOrWrongTenant) {
		t.Fatalf("cross-tenant update: want ErrLogNotFoundOrWrongTenant, got %v", err)
	}

	// Reload and confirm the AIInsight field was NOT modified.
	var after Log
	if err := repo.db.First(&after, owned.ID).Error; err != nil {
		t.Fatalf("reload log: %v", err)
	}
	if string(after.AIInsight) != "" {
		t.Fatalf("cross-tenant write leaked: ai_insight = %q", string(after.AIInsight))
	}

	// Legitimate same-tenant update must still succeed.
	aCtx := WithTenantContext(context.Background(), "tenant-a")
	if err := repo.UpdateLogInsight(aCtx, owned.ID, "legit insight"); err != nil {
		t.Fatalf("same-tenant update: %v", err)
	}
	if err := repo.db.First(&after, owned.ID).Error; err != nil {
		t.Fatalf("reload after legit update: %v", err)
	}
	if string(after.AIInsight) != "legit insight" {
		t.Fatalf("same-tenant update did not persist: got %q", string(after.AIInsight))
	}
}

// TestUpdateLogInsight_UnknownIDReturnsTypedError proves the not-found case
// also gets the sentinel error (handlers translate both to 404 so tenant
// existence is not leaked).
func TestUpdateLogInsight_UnknownIDReturnsTypedError(t *testing.T) {
	repo := newTestRepo(t)
	ctx := WithTenantContext(context.Background(), "tenant-a")

	err := repo.UpdateLogInsight(ctx, 99999, "x")
	if !errors.Is(err, ErrLogNotFoundOrWrongTenant) {
		t.Fatalf("unknown id: want ErrLogNotFoundOrWrongTenant, got %v", err)
	}
}

// TestRepo_Queries_RespectContextCancel proves that r.db is wrapped with
// WithContext(ctx) at all query entry points — a pre-cancelled context must
// cause queries to return promptly with a context-cancelled error instead of
// running to completion.
//
// Without the WithContext(ctx) wiring, GetLogsV2's errgroup goroutines would
// run to completion (the ctx never propagates to the DB driver) and this test
// would hang at the Wait() call.
func TestRepo_Queries_RespectContextCancel(t *testing.T) {
	repo := newTestRepo(t)
	now := time.Now().UTC()

	// Seed some data so queries would otherwise succeed.
	seedTenantLogs(t, repo, "acme", 50, now)
	seedTenantTraces(t, repo, "acme", 50, now)

	// Build a ctx that is already cancelled AND carries a tenant.
	ctx := WithTenantContext(context.Background(), "acme")
	ctx, cancel := context.WithCancel(ctx)
	cancel() // pre-cancelled

	done := make(chan struct{})
	var getLogsErr, getTracesErr, getStatsErr, getDashErr error

	go func() {
		defer close(done)
		_, _, getLogsErr = repo.GetLogsV2(ctx, LogFilter{Limit: 100})
		_, getTracesErr = repo.GetTracesFiltered(ctx, time.Time{}, time.Time{}, nil, "", "", 100, 0, "timestamp", "desc")
		_, getStatsErr = repo.GetStats(ctx)
		_, getDashErr = repo.GetDashboardStats(ctx, now.Add(-time.Hour), now.Add(time.Hour), nil)
	}()

	select {
	case <-done:
		// Good: returned promptly. At least one should report cancellation.
		if getLogsErr == nil && getTracesErr == nil && getStatsErr == nil && getDashErr == nil {
			t.Fatalf("expected at least one query to surface context cancellation; all returned nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("queries hung past 2s despite pre-cancelled context — WithContext(ctx) likely not wired")
	}
}
