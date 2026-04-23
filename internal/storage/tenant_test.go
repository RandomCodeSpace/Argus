package storage

import (
	"context"
	"testing"
	"time"
)

func TestLog_TenantID_PersistsExplicitValue(t *testing.T) {
	repo := newTestRepo(t)
	now := time.Now().UTC()

	want := Log{
		TenantID:    "acme",
		TraceID:     "t-001",
		SpanID:      "s-001",
		Severity:    "INFO",
		Body:        "hello acme",
		ServiceName: "svc-a",
		Timestamp:   now,
	}
	if err := repo.db.Create(&want).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}

	var got Log
	if err := repo.db.First(&got, want.ID).Error; err != nil {
		t.Fatalf("First: %v", err)
	}
	if got.TenantID != "acme" {
		t.Fatalf("TenantID not persisted: want=acme got=%q", got.TenantID)
	}
}

func TestLog_TenantID_DefaultsToDefaultWhenUnset(t *testing.T) {
	repo := newTestRepo(t)
	now := time.Now().UTC()

	// No explicit TenantID. GORM default:'default' should fill it.
	// We go through raw SQL to bypass Go's zero-value passthrough and let the DB default apply.
	if err := repo.db.Exec(
		`INSERT INTO logs (trace_id, span_id, severity, body, service_name, timestamp) VALUES (?, ?, ?, ?, ?, ?)`,
		"t-002", "s-002", "INFO", "defaulted", "svc-b", now,
	).Error; err != nil {
		t.Fatalf("raw insert: %v", err)
	}

	var got Log
	if err := repo.db.Where("trace_id = ?", "t-002").First(&got).Error; err != nil {
		t.Fatalf("First: %v", err)
	}
	if got.TenantID != DefaultTenantID {
		t.Fatalf("TenantID default not applied: want=%q got=%q", DefaultTenantID, got.TenantID)
	}
}

func TestSpan_TraceID_MetricBucket_TenantIDPersisted(t *testing.T) {
	repo := newTestRepo(t)
	now := time.Now().UTC()

	tr := Trace{TenantID: "acme", TraceID: "tr-001", ServiceName: "svc", Timestamp: now, Status: "OK"}
	if err := repo.db.Create(&tr).Error; err != nil {
		t.Fatalf("trace: %v", err)
	}
	sp := Span{TenantID: "acme", TraceID: "tr-001", SpanID: "sp-001", OperationName: "op", ServiceName: "svc", StartTime: now, EndTime: now}
	if err := repo.db.Create(&sp).Error; err != nil {
		t.Fatalf("span: %v", err)
	}
	mb := MetricBucket{TenantID: "acme", Name: "m", ServiceName: "svc", TimeBucket: now, Count: 1, Sum: 1, Min: 1, Max: 1}
	if err := repo.db.Create(&mb).Error; err != nil {
		t.Fatalf("mb: %v", err)
	}

	var gotTrace Trace
	var gotSpan Span
	var gotMB MetricBucket
	if err := repo.db.Where("trace_id = ?", "tr-001").First(&gotTrace).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Where("span_id = ?", "sp-001").First(&gotSpan).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.db.Where("name = ?", "m").First(&gotMB).Error; err != nil {
		t.Fatal(err)
	}
	if gotTrace.TenantID != "acme" || gotSpan.TenantID != "acme" || gotMB.TenantID != "acme" {
		t.Fatalf("tenants not persisted: trace=%q span=%q mb=%q", gotTrace.TenantID, gotSpan.TenantID, gotMB.TenantID)
	}
}

// TestTenantFromContext_FallsBackToDefault proves that a context without a
// tenant value (or a nil context) resolves to DefaultTenantID. This is the
// behaviour single-tenant installs rely on.
func TestTenantFromContext_FallsBackToDefault(t *testing.T) {
	if got := TenantFromContext(context.Background()); got != DefaultTenantID {
		t.Fatalf("empty ctx: want %q got %q", DefaultTenantID, got)
	}
	//nolint:staticcheck // intentional nil context to prove the fallback path
	if got := TenantFromContext(nil); got != DefaultTenantID {
		t.Fatalf("nil ctx: want %q got %q", DefaultTenantID, got)
	}
	// Explicit empty string should also fall back.
	ctx := WithTenantContext(context.Background(), "")
	if got := TenantFromContext(ctx); got != DefaultTenantID {
		t.Fatalf("empty string ctx: want %q got %q", DefaultTenantID, got)
	}
	// Explicit value should be preserved.
	ctx = WithTenantContext(context.Background(), "acme")
	if got := TenantFromContext(ctx); got != "acme" {
		t.Fatalf("explicit ctx: want %q got %q", "acme", got)
	}
}

// TestGetLogs_ScopedByTenant proves that GetLogsV2 never returns rows from
// a tenant other than the one on the request context.
func TestGetLogs_ScopedByTenant(t *testing.T) {
	repo := newTestRepo(t)
	now := time.Now().UTC()

	seedTenantLogs(t, repo, "acme", 3, now)
	seedTenantLogs(t, repo, "globex", 2, now)

	acmeCtx := WithTenantContext(context.Background(), "acme")
	globexCtx := WithTenantContext(context.Background(), "globex")
	defaultCtx := context.Background()

	acme, total, err := repo.GetLogsV2(acmeCtx, LogFilter{Limit: 100})
	if err != nil {
		t.Fatalf("GetLogsV2(acme): %v", err)
	}
	if total != 3 || len(acme) != 3 {
		t.Fatalf("acme tenant: want 3 logs (total=3), got %d (total=%d)", len(acme), total)
	}
	for _, l := range acme {
		if l.TenantID != "acme" {
			t.Fatalf("acme leak: got tenant %q", l.TenantID)
		}
	}

	globex, total, err := repo.GetLogsV2(globexCtx, LogFilter{Limit: 100})
	if err != nil {
		t.Fatalf("GetLogsV2(globex): %v", err)
	}
	if total != 2 || len(globex) != 2 {
		t.Fatalf("globex tenant: want 2 logs (total=2), got %d (total=%d)", len(globex), total)
	}

	// Default-tenant ctx must see neither acme nor globex.
	def, total, err := repo.GetLogsV2(defaultCtx, LogFilter{Limit: 100})
	if err != nil {
		t.Fatalf("GetLogsV2(default): %v", err)
	}
	if total != 0 || len(def) != 0 {
		t.Fatalf("default tenant should be empty, got %d rows (total=%d)", len(def), total)
	}
}

// TestGetTracesFiltered_ScopedByTenant proves the same isolation for traces.
func TestGetTracesFiltered_ScopedByTenant(t *testing.T) {
	repo := newTestRepo(t)
	now := time.Now().UTC()

	seedTenantTraces(t, repo, "acme", 4, now)
	seedTenantTraces(t, repo, "globex", 1, now)

	acmeCtx := WithTenantContext(context.Background(), "acme")
	globexCtx := WithTenantContext(context.Background(), "globex")

	resp, err := repo.GetTracesFiltered(acmeCtx, time.Time{}, time.Time{}, nil, "", "", 100, 0, "timestamp", "desc")
	if err != nil {
		t.Fatalf("GetTracesFiltered(acme): %v", err)
	}
	if resp.Total != 4 {
		t.Fatalf("acme traces: want 4 got %d", resp.Total)
	}
	for _, tr := range resp.Traces {
		if tr.TenantID != "acme" {
			t.Fatalf("acme leak: got tenant %q", tr.TenantID)
		}
	}

	resp, err = repo.GetTracesFiltered(globexCtx, time.Time{}, time.Time{}, nil, "", "", 100, 0, "timestamp", "desc")
	if err != nil {
		t.Fatalf("GetTracesFiltered(globex): %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("globex traces: want 1 got %d", resp.Total)
	}
}

// TestGetDashboardStats_ScopedByTenant proves dashboard totals are scoped.
func TestGetDashboardStats_ScopedByTenant(t *testing.T) {
	repo := newTestRepo(t)
	now := time.Now().UTC()

	seedTenantTraces(t, repo, "acme", 5, now)
	seedTenantTraces(t, repo, "globex", 2, now)
	seedTenantLogs(t, repo, "acme", 3, now)
	seedTenantLogs(t, repo, "globex", 7, now)

	start := now.Add(-1 * time.Minute)
	end := now.Add(1 * time.Minute)

	acmeCtx := WithTenantContext(context.Background(), "acme")
	globexCtx := WithTenantContext(context.Background(), "globex")

	acme, err := repo.GetDashboardStats(acmeCtx, start, end, nil)
	if err != nil {
		t.Fatalf("GetDashboardStats(acme): %v", err)
	}
	if acme.TotalTraces != 5 || acme.TotalLogs != 3 {
		t.Fatalf("acme stats: want traces=5 logs=3, got traces=%d logs=%d", acme.TotalTraces, acme.TotalLogs)
	}

	globex, err := repo.GetDashboardStats(globexCtx, start, end, nil)
	if err != nil {
		t.Fatalf("GetDashboardStats(globex): %v", err)
	}
	if globex.TotalTraces != 2 || globex.TotalLogs != 7 {
		t.Fatalf("globex stats: want traces=2 logs=7, got traces=%d logs=%d", globex.TotalTraces, globex.TotalLogs)
	}
}

// seedTenantLogs inserts n logs for the given tenant.
func seedTenantLogs(t *testing.T, repo *Repository, tenant string, n int, ts time.Time) {
	t.Helper()
	logs := make([]Log, n)
	for i := 0; i < n; i++ {
		logs[i] = Log{
			TenantID:    tenant,
			TraceID:     "t-" + tenant,
			SpanID:      "s-" + tenant,
			Severity:    "INFO",
			Body:        "log for " + tenant,
			ServiceName: "svc-" + tenant,
			Timestamp:   ts,
		}
	}
	if err := repo.db.CreateInBatches(logs, 500).Error; err != nil {
		t.Fatalf("seedTenantLogs: %v", err)
	}
}

// seedTenantTraces inserts n traces for the given tenant.
func seedTenantTraces(t *testing.T, repo *Repository, tenant string, n int, ts time.Time) {
	t.Helper()
	traces := make([]Trace, n)
	for i := 0; i < n; i++ {
		traces[i] = Trace{
			TenantID:    tenant,
			TraceID:     tenant + "-tr-" + time.Now().Format("150405.000000000") + "-" + itoa(i),
			ServiceName: "svc-" + tenant,
			Duration:    1000,
			Status:      "OK",
			Timestamp:   ts,
		}
	}
	if err := repo.db.CreateInBatches(traces, 500).Error; err != nil {
		t.Fatalf("seedTenantTraces: %v", err)
	}
}

// itoa avoids pulling strconv just for tests.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	n := i
	if n < 0 {
		n = -n
	}
	for n > 0 {
		pos--
		b[pos] = byte('0' + n%10)
		n /= 10
	}
	if i < 0 {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
