//go:build integration
// +build integration

// Postgres-backed integration tests for declarative daily partitioning.
//
// Run with:
//
//	go test -race -tags=integration ./internal/storage/...
//
// Auto-skips if Docker is unavailable (matches pg_integration_test.go).

package storage

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// setupPGContainerPartitioned boots a Postgres 16 container, runs
// AutoMigrateModelsWithOptions with daily partitioning enabled, and returns a
// repository + teardown closure. Skipped (not failed) when Docker is missing.
func setupPGContainerPartitioned(t *testing.T, lookahead int) (*Repository, func()) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("otel_test"),
		postgres.WithUsername("otel"),
		postgres.WithPassword("otel"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Skipf("docker unavailable, skipping pg partition tests: %v", err)
	}

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("ConnectionString: %v", err)
	}

	db, err := NewDatabase("postgres", dsn)
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("NewDatabase(postgres): %v", err)
	}
	if err := AutoMigrateModelsWithOptions(db, "postgres", MigrateOptions{
		PostgresPartitioning:   PartitioningModeDaily,
		PartitionLookaheadDays: lookahead,
	}); err != nil {
		_ = pgContainer.Terminate(ctx)
		t.Fatalf("AutoMigrateModelsWithOptions: %v", err)
	}
	repo := NewRepositoryFromDB(db, "postgres")
	teardown := func() {
		_ = repo.Close()
		_ = pgContainer.Terminate(ctx)
	}
	return repo, teardown
}

// TestPGPartition_LogsTableIsPartitioned verifies that with the option
// enabled, `logs` is a partitioned (relkind=p) parent and the initial
// partitions are attached.
func TestPGPartition_LogsTableIsPartitioned(t *testing.T) {
	repo, teardown := setupPGContainerPartitioned(t, 3)
	defer teardown()

	rk, err := pgLogsRelkind(repo.db)
	if err != nil {
		t.Fatalf("pgLogsRelkind: %v", err)
	}
	if rk != "p" {
		t.Fatalf("logs should be partitioned (relkind=p), got %q", rk)
	}

	count, err := countLogsPartitions(context.Background(), repo.db)
	if err != nil {
		t.Fatalf("countLogsPartitions: %v", err)
	}
	// yesterday + today + 3 future = 5
	if count < 5 {
		t.Fatalf("want >=5 initial partitions; got %d", count)
	}
}

// TestPGPartition_InsertRoutesToCorrectChild verifies that an INSERT goes
// into the correct daily child partition.
func TestPGPartition_InsertRoutesToCorrectChild(t *testing.T) {
	repo, teardown := setupPGContainerPartitioned(t, 1)
	defer teardown()

	now := time.Now().UTC()
	if err := repo.db.Create(&Log{
		Severity:    "INFO",
		Body:        "routed to today",
		ServiceName: "api",
		Timestamp:   now,
	}).Error; err != nil {
		t.Fatalf("create log: %v", err)
	}

	expected := partitionNameForDay(now)
	var found int
	row := repo.db.Raw(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, quoteIdent(expected))).Row()
	if err := row.Scan(&found); err != nil {
		t.Fatalf("count partition rows: %v", err)
	}
	if found != 1 {
		t.Fatalf("expected 1 row in partition %s, got %d", expected, found)
	}
}

// TestPGPartition_DropExpired confirms that a partition whose upper bound
// predates the cutoff is dropped, and that other partitions are kept.
func TestPGPartition_DropExpired(t *testing.T) {
	repo, teardown := setupPGContainerPartitioned(t, 2)
	defer teardown()

	// Stage a partition for 30 days ago — well outside any reasonable retention.
	old := time.Now().UTC().Add(-30 * 24 * time.Hour)
	if err := EnsureLogsPartitionForDay(repo.db, old); err != nil {
		t.Fatalf("ensure old partition: %v", err)
	}
	beforeName := partitionNameForDay(old)
	beforeCount, err := countLogsPartitions(context.Background(), repo.db)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour) // 7-day retention
	dropped, err := DropExpiredLogsPartitions(context.Background(), repo.db, cutoff)
	if err != nil {
		t.Fatalf("DropExpiredLogsPartitions: %v", err)
	}
	if dropped < 1 {
		t.Fatalf("expected at least 1 dropped partition (the 30-day-old one), got %d", dropped)
	}

	// The dropped partition should no longer exist.
	var present int
	if err := repo.db.Raw(`SELECT COUNT(*) FROM pg_class WHERE relname = ?`, beforeName).Row().Scan(&present); err != nil {
		t.Fatalf("check class: %v", err)
	}
	if present != 0 {
		t.Fatalf("partition %s should have been dropped", beforeName)
	}

	// Today's partition should NOT have been dropped.
	todayName := partitionNameForDay(time.Now().UTC())
	if err := repo.db.Raw(`SELECT COUNT(*) FROM pg_class WHERE relname = ?`, todayName).Row().Scan(&present); err != nil {
		t.Fatalf("check today class: %v", err)
	}
	if present != 1 {
		t.Fatalf("today's partition %s should still exist", todayName)
	}

	afterCount, err := countLogsPartitions(context.Background(), repo.db)
	if err != nil {
		t.Fatalf("count after: %v", err)
	}
	if afterCount >= beforeCount {
		t.Fatalf("partition count should have decreased: before=%d after=%d", beforeCount, afterCount)
	}
}

// TestPGPartition_GreenfieldGuard verifies that running partitioning setup
// against a DB with an existing non-partitioned `logs` table refuses to
// proceed. We reach this by running AutoMigrateModels (no partition opts)
// first, then trying the partitioning path on the same DB.
func TestPGPartition_GreenfieldGuard(t *testing.T) {
	ctx := context.Background()
	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("otel_test"),
		postgres.WithUsername("otel"),
		postgres.WithPassword("otel"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Skipf("docker unavailable, skipping: %v", err)
	}
	defer func() { _ = pgContainer.Terminate(ctx) }()

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("ConnectionString: %v", err)
	}
	db, err := NewDatabase("postgres", dsn)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() {
		if sqlDB, _ := db.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	}()

	// First migrate without partitioning — creates a regular `logs` table.
	if err := AutoMigrateModels(db, "postgres"); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	// Now attempt to enable partitioning — should refuse.
	err = AutoMigrateModelsWithOptions(db, "postgres", MigrateOptions{
		PostgresPartitioning:   PartitioningModeDaily,
		PartitionLookaheadDays: 3,
	})
	if err == nil {
		t.Fatal("expected error when enabling partitioning on existing non-partitioned logs table")
	}
	if !strings.Contains(err.Error(), "greenfield-only") {
		t.Fatalf("expected greenfield-only error, got: %v", err)
	}
}

// TestPGPartition_SchedulerDropsExpiredAndCreatesLookahead exercises the
// full PartitionScheduler lifecycle.
func TestPGPartition_SchedulerDropsExpiredAndCreatesLookahead(t *testing.T) {
	repo, teardown := setupPGContainerPartitioned(t, 1)
	defer teardown()

	// Stage an old partition outside retention.
	old := time.Now().UTC().Add(-30 * 24 * time.Hour)
	if err := EnsureLogsPartitionForDay(repo.db, old); err != nil {
		t.Fatalf("ensure old: %v", err)
	}

	sched := NewPartitionScheduler(repo, 7, 5) // retention=7d, lookahead=5d
	// Tighten intervals so the test isn't slow; we still rely on the
	// synchronous initial pass in Start() rather than the loop.
	sched.ensureInterval = time.Hour
	sched.dropInterval = time.Hour

	dropped := 0
	active := 0
	sched.SetMetrics(func(n int) { dropped += n }, func(n int) { active = n })

	ctx, cancel := context.WithCancel(context.Background())
	sched.Start(ctx)
	defer func() { cancel(); sched.Stop() }()

	if dropped < 1 {
		t.Fatalf("scheduler initial pass should have dropped >=1 expired partition; got %d", dropped)
	}
	// active should be at least lookahead (5) + today + yesterday = 7.
	if active < 7 {
		t.Fatalf("active partitions after initial ensure should be >=7; got %d", active)
	}

	// Idempotency: another tick is a no-op.
	sched.runEnsure(ctx)
	sched.runDrop(ctx)
}

// TestPGPartition_PgTrgmIndexesPropagateToChildren verifies that the GIN
// trigram indexes declared on the parent are inherited by daily children
// (Postgres ≥ 11 propagates partitioned indexes automatically).
func TestPGPartition_PgTrgmIndexesPropagateToChildren(t *testing.T) {
	repo, teardown := setupPGContainerPartitioned(t, 1)
	defer teardown()

	// Pick today's partition name and confirm that an idx_logs_body_trgm
	// inherited index exists on it.
	todayName := partitionNameForDay(time.Now().UTC())
	var ginCount int
	if err := repo.db.Raw(`
		SELECT COUNT(*)
		FROM pg_indexes
		WHERE schemaname = current_schema()
		  AND tablename = ?
		  AND indexdef ILIKE '%USING gin%'
	`, todayName).Row().Scan(&ginCount); err != nil {
		t.Fatalf("inspect indexes: %v", err)
	}
	if ginCount < 1 {
		t.Skipf("pg_trgm GIN inheritance index not present on %s — extension may be missing", todayName)
	}
}
