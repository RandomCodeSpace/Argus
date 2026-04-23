package telemetry

import (
	"database/sql"
	"testing"

	_ "github.com/glebarez/go-sqlite" // registers "sqlite" driver used by glebarez/sqlite GORM dialect
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// gaugeValueForTest scrapes the current value of a Prometheus gauge.
func gaugeValueForTest(t *testing.T, g prometheus.Gauge) float64 {
	t.Helper()
	var m dto.Metric
	if err := g.Write(&m); err != nil {
		t.Fatalf("gauge write: %v", err)
	}
	return m.GetGauge().GetValue()
}

// TestSampleDBPoolStats exercises both the happy path and nil safety. It is
// a single test because telemetry.New() registers metrics against the global
// Prometheus default registry via promauto — calling it twice in the same
// test binary panics with a duplicate-collector error. Using one New() plus
// subtests keeps coverage without colliding with the global registry.
func TestSampleDBPoolStats(t *testing.T) {
	m := New()

	t.Run("WritesGauges", func(t *testing.T) {
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatalf("sql.Open: %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })

		// Force a connection so the pool reports something.
		if err := db.Ping(); err != nil {
			t.Fatalf("ping: %v", err)
		}

		m.SampleDBPoolStats(db)

		stats := db.Stats()
		if got := gaugeValueForTest(t, m.DBPoolOpenConnections); got != float64(stats.OpenConnections) {
			t.Fatalf("open_connections: got %v want %v", got, stats.OpenConnections)
		}
		if got := gaugeValueForTest(t, m.DBPoolInUse); got != float64(stats.InUse) {
			t.Fatalf("in_use: got %v want %v", got, stats.InUse)
		}
		if got := gaugeValueForTest(t, m.DBPoolIdle); got != float64(stats.Idle) {
			t.Fatalf("idle: got %v want %v", got, stats.Idle)
		}
		// wait_count and wait_duration start at 0 on a fresh pool; just verify
		// the gauges exist and are readable without error.
		_ = gaugeValueForTest(t, m.DBPoolWaitCount)
		_ = gaugeValueForTest(t, m.DBPoolWaitDuration)
	})

	t.Run("NilSafe", func(t *testing.T) {
		// nil *sql.DB must not panic.
		m.SampleDBPoolStats(nil)

		// nil receiver must not panic either.
		var m2 *Metrics
		m2.SampleDBPoolStats(nil)
	})
}
