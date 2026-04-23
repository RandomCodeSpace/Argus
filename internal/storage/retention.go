package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RetentionScheduler periodically enforces hot-DB retention and runs DB maintenance.
// On startup and hourly thereafter it deletes rows older than retentionDays.
// Daily it runs driver-appropriate maintenance (VACUUM ANALYZE / OPTIMIZE / VACUUM).
type RetentionScheduler struct {
	repo           *Repository
	retentionDays  int
	purgeInterval  time.Duration
	vacuumInterval time.Duration
	purgeBatchSize int

	// started is an atomic so a fast-path Stop() before Start() is lock-free.
	// mu serializes the Start/Stop transition itself (protects cancel + done).
	started atomic.Bool
	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}

	// running prevents overlapping purge/maintenance passes. If a run exceeds
	// purgeInterval, the next tick is skipped with a warn rather than piling on
	// contention (and potentially holding a long-running DELETE behind another).
	running atomic.Bool

	// skippedRuns increments every time a tick is dropped because running==true.
	// Test hook; exported via SkippedRuns().
	skippedRuns atomic.Int64
}

// NewRetentionScheduler constructs a scheduler but does not start it.
func NewRetentionScheduler(repo *Repository, retentionDays int) *RetentionScheduler {
	return &RetentionScheduler{
		repo:           repo,
		retentionDays:  retentionDays,
		purgeInterval:  1 * time.Hour,
		vacuumInterval: 24 * time.Hour,
		purgeBatchSize: 10_000,
		done:           make(chan struct{}),
	}
}

// SkippedRuns returns the number of purge/maintenance ticks that were dropped
// because a previous run was still executing. Intended for tests and telemetry.
func (r *RetentionScheduler) SkippedRuns() int64 { return r.skippedRuns.Load() }

// Start launches the scheduler goroutine. It runs an initial purge immediately.
// Idempotent and race-free: atomic CAS elects the first caller, and mu
// publishes cancel+done before any concurrent Stop can observe started=true.
func (r *RetentionScheduler) Start(parent context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started.Load() {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	go r.loop(ctx)
	r.started.Store(true)
}

// Stop signals the scheduler to exit and waits for the loop to return.
// No-op if Start was never called. Safe to call concurrently / repeatedly.
func (r *RetentionScheduler) Stop() {
	if !r.started.Load() {
		return
	}
	r.mu.Lock()
	cancel := r.cancel
	done := r.done
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (r *RetentionScheduler) loop(ctx context.Context) {
	defer close(r.done)

	purgeTick := time.NewTicker(r.purgeInterval)
	defer purgeTick.Stop()
	vacuumTick := time.NewTicker(r.vacuumInterval)
	defer vacuumTick.Stop()

	// Run an initial purge pass at startup so a long-paused instance catches up quickly.
	r.runPurge(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-purgeTick.C:
			r.runPurge(ctx)
		case <-vacuumTick.C:
			r.runMaintenance(ctx)
		}
	}
}

func (r *RetentionScheduler) runPurge(ctx context.Context) {
	// Overlap guard: if a previous purge/maintenance is still in flight, skip.
	if !r.running.CompareAndSwap(false, true) {
		r.skippedRuns.Add(1)
		slog.Warn("retention: previous run still in progress, skipping this tick", "phase", "purge")
		return
	}
	defer r.running.Store(false)

	driver := strings.ToLower(r.repo.driver)
	if driver == "" {
		driver = "sqlite"
	}
	metrics := r.repo.metrics

	start := time.Now()
	cutoff := time.Now().UTC().Add(-time.Duration(r.retentionDays) * 24 * time.Hour)

	// Fix 6: track failure across all three purges so we can expose
	// retention_consecutive_failures{job="purge"} accurately.
	purgeFailed := false

	logs, err := r.repo.PurgeLogsBatched(ctx, cutoff, r.purgeBatchSize)
	if err != nil {
		slog.Error("retention: purge logs failed", "error", err)
		purgeFailed = true
	}
	if metrics != nil && logs > 0 {
		metrics.RetentionRowsPurgedTotal.WithLabelValues("logs", driver).Add(float64(logs))
	}

	traces, err := r.repo.PurgeTracesBatched(ctx, cutoff, r.purgeBatchSize)
	if err != nil {
		slog.Error("retention: purge traces failed", "error", err)
		purgeFailed = true
	}
	if metrics != nil && traces > 0 {
		// PurgeTracesBatched deletes traces and sweeps orphan spans. The returned
		// count reflects traces; report under the "traces" label. Spans are swept
		// as a side effect — no separate authoritative count is returned.
		metrics.RetentionRowsPurgedTotal.WithLabelValues("traces", driver).Add(float64(traces))
	}

	metricsPurged, err := r.repo.PurgeMetricBucketsBatched(ctx, cutoff, r.purgeBatchSize)
	if err != nil {
		slog.Error("retention: purge metrics failed", "error", err)
		purgeFailed = true
	}
	if metrics != nil && metricsPurged > 0 {
		metrics.RetentionRowsPurgedTotal.WithLabelValues("metric_buckets", driver).Add(float64(metricsPurged))
	}

	if metrics != nil {
		metrics.RetentionPurgeDurationSeconds.WithLabelValues(driver).Observe(time.Since(start).Seconds())
		if purgeFailed {
			metrics.RetentionConsecutiveFailures.WithLabelValues("purge").Inc()
		} else {
			metrics.RetentionConsecutiveFailures.WithLabelValues("purge").Set(0)
			metrics.RetentionLastSuccessTimestamp.WithLabelValues("purge").Set(float64(time.Now().Unix()))
		}
	}

	slog.Info("retention purge complete",
		"cutoff", cutoff.Format(time.RFC3339),
		"logs_deleted", logs,
		"traces_deleted", traces,
		"metrics_deleted", metricsPurged,
		"duration", time.Since(start),
	)
}

func (r *RetentionScheduler) runMaintenance(ctx context.Context) {
	if !r.running.CompareAndSwap(false, true) {
		r.skippedRuns.Add(1)
		slog.Warn("retention: previous run still in progress, skipping this tick", "phase", "maintenance")
		return
	}
	defer r.running.Store(false)

	driver := strings.ToLower(r.repo.driver)
	if driver == "" {
		driver = "sqlite"
	}
	metrics := r.repo.metrics

	// Fix 6: track whether any step failed so we can set the right gauge.
	maintFailed := false
	defer func() {
		if metrics == nil {
			return
		}
		if maintFailed {
			metrics.RetentionConsecutiveFailures.WithLabelValues("maintenance").Inc()
			return
		}
		metrics.RetentionConsecutiveFailures.WithLabelValues("maintenance").Set(0)
		metrics.RetentionLastSuccessTimestamp.WithLabelValues("maintenance").Set(float64(time.Now().Unix()))
	}()

	// VACUUM cannot run inside a transaction on Postgres or SQLite.
	// GORM's db.Exec wraps statements in an implicit tx, so we drop to the raw *sql.DB.
	sqlDB, err := r.repo.db.DB()
	if err != nil {
		slog.Error("retention: get raw sql.DB failed", "error", err)
		maintFailed = true
		return
	}

	observe := func(table string, d time.Duration) {
		if metrics != nil {
			metrics.RetentionVacuumDurationSeconds.WithLabelValues(driver, table).Observe(d.Seconds())
		}
	}

	switch driver {
	case "postgres", "postgresql":
		for _, t := range []string{"logs", "spans", "traces", "metric_buckets"} {
			start := time.Now()
			if _, err := sqlDB.ExecContext(ctx, fmt.Sprintf("VACUUM ANALYZE %s", t)); err != nil {
				slog.Error("retention: VACUUM ANALYZE failed", "table", t, "error", err)
				maintFailed = true
			}
			observe(t, time.Since(start))
		}
	case "mysql":
		// OPTIMIZE TABLE can run through the gorm handle (no tx restriction).
		db := r.repo.db.WithContext(ctx)
		for _, t := range []string{"logs", "spans", "traces", "metric_buckets"} {
			start := time.Now()
			if err := db.Exec(fmt.Sprintf("OPTIMIZE TABLE %s", t)).Error; err != nil {
				slog.Error("retention: OPTIMIZE TABLE failed", "table", t, "error", err)
				maintFailed = true
			}
			observe(t, time.Since(start))
		}
	case "sqlite":
		start := time.Now()
		if _, err := sqlDB.ExecContext(ctx, "PRAGMA optimize"); err != nil {
			slog.Error("retention: PRAGMA optimize failed", "error", err)
			maintFailed = true
		}
		if _, err := sqlDB.ExecContext(ctx, "VACUUM"); err != nil {
			slog.Error("retention: VACUUM failed", "error", err)
			maintFailed = true
		}
		// SQLite VACUUM is whole-DB; record a single observation under "all".
		observe("all", time.Since(start))
	}
	slog.Info("retention maintenance complete", "driver", driver)
}
