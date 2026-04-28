package storage

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

// PartitionScheduler maintains daily logs partitions on Postgres when
// DB_POSTGRES_PARTITIONING=daily is enabled. Hourly it ensures the next
// `lookaheadDays` partitions exist; daily it drops partitions whose upper
// bound predates the retention cutoff. Both passes are idempotent so a
// stalled tick (or a parallel scheduler from a different replica) is safe.
//
// The scheduler is independent of RetentionScheduler so the legacy DELETE
// path (used for SQLite/MySQL/MSSQL or non-partitioned Postgres) keeps
// running on its own loop. When partitioning is enabled, RetentionScheduler
// SHOULD skip logs — wire that up at construction time, not here.
type PartitionScheduler struct {
	repo            *Repository
	retentionDays   int
	lookaheadDays   int
	ensureInterval  time.Duration
	dropInterval    time.Duration
	onPartitionDrop func(int) // metric callback: count of partitions dropped
	onPartitionKeep func(int) // metric callback: count of partitions still active

	started atomic.Bool
	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewPartitionScheduler constructs a scheduler. retentionDays must match the
// HOT_RETENTION_DAYS setting so DROP PARTITION is the moral equivalent of the
// hourly DELETE-by-age the RetentionScheduler runs for non-partitioned tables.
func NewPartitionScheduler(repo *Repository, retentionDays, lookaheadDays int) *PartitionScheduler {
	if retentionDays < 1 {
		retentionDays = 7
	}
	if lookaheadDays < 1 {
		lookaheadDays = 3
	}
	return &PartitionScheduler{
		repo:           repo,
		retentionDays:  retentionDays,
		lookaheadDays:  lookaheadDays,
		ensureInterval: 1 * time.Hour,
		dropInterval:   1 * time.Hour,
		done:           make(chan struct{}),
	}
}

// SetMetrics wires telemetry callbacks. Both arguments may be nil.
func (s *PartitionScheduler) SetMetrics(onDrop, onKeep func(int)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onPartitionDrop = onDrop
	s.onPartitionKeep = onKeep
}

// Start kicks off the background loop. It performs an initial ensure+drop
// pass synchronously so a fresh boot has the next-day partition staged
// before any ingest hits it.
//
// One-shot lifecycle: Start is idempotent (a second call while running is a
// no-op), and a Start-Stop-Start sequence is NOT supported — Stop closes
// the internal `done` channel, and re-running Start would re-close it
// during shutdown of the second iteration. Construct a fresh
// PartitionScheduler if you need to restart.
func (s *PartitionScheduler) Start(parent context.Context) {
	s.mu.Lock()
	if s.started.Load() {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	s.started.Store(true)
	s.mu.Unlock()

	// Initial pass — synchronous so the operator sees the partition layout
	// before the binary becomes ready.
	s.runEnsure(ctx)
	s.runDrop(ctx)

	go s.loop(ctx)
}

// Stop cancels the loop and waits for it to exit. Safe to call multiple times.
func (s *PartitionScheduler) Stop() {
	if !s.started.Load() {
		return
	}
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *PartitionScheduler) loop(ctx context.Context) {
	defer close(s.done)
	ensureTick := time.NewTicker(s.ensureInterval)
	dropTick := time.NewTicker(s.dropInterval)
	defer ensureTick.Stop()
	defer dropTick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ensureTick.C:
			s.runEnsure(ctx)
		case <-dropTick.C:
			s.runDrop(ctx)
		}
	}
}

func (s *PartitionScheduler) runEnsure(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	if _, err := EnsureLogsLookahead(s.repo.db.WithContext(ctx), s.lookaheadDays); err != nil {
		slog.Error("partition scheduler: ensure failed", "err", err)
	}
}

func (s *PartitionScheduler) runDrop(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	cutoff := time.Now().UTC().Add(-time.Duration(s.retentionDays) * 24 * time.Hour)
	dropped, err := DropExpiredLogsPartitions(ctx, s.repo.db, cutoff)
	if err != nil {
		slog.Error("partition scheduler: drop failed", "err", err)
		return
	}
	if dropped > 0 && s.onPartitionDrop != nil {
		s.onPartitionDrop(dropped)
	}
	if s.onPartitionKeep != nil {
		count, _ := countLogsPartitions(ctx, s.repo.db)
		s.onPartitionKeep(count)
	}
}

// countLogsPartitions returns the current number of partitions attached to
// the `logs` parent. Used for the gauge so operators can spot a stuck loop
// (count keeps growing) or an over-aggressive drop (count keeps shrinking).
func countLogsPartitions(ctx context.Context, db *gorm.DB) (int, error) {
	var n int
	err := db.WithContext(ctx).Raw(`
		SELECT COUNT(*)
		FROM pg_class p
		JOIN pg_inherits i ON i.inhparent = p.oid
		WHERE p.relname = 'logs' AND p.relkind = 'p'
	`).Row().Scan(&n)
	return n, err
}
