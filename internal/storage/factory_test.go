package storage

import (
	"strings"
	"testing"
)

func TestNewDatabase_UnsupportedDriver(t *testing.T) {
	_, err := NewDatabase("mongodb", "whatever")
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("want unsupported-driver error, got %v", err)
	}
}

func TestNewDatabase_PostgresRequiresDSN(t *testing.T) {
	_, err := NewDatabase("postgres", "")
	if err == nil || !strings.Contains(err.Error(), "DB_DSN is required") {
		t.Fatalf("postgres without DSN must error; got %v", err)
	}
	_, err = NewDatabase("postgresql", "")
	if err == nil {
		t.Fatal("postgresql alias should also require DSN")
	}
}

func TestNewDatabase_SQLServerRequiresDSN(t *testing.T) {
	_, err := NewDatabase("sqlserver", "")
	if err == nil || !strings.Contains(err.Error(), "DB_DSN is required") {
		t.Fatalf("want DSN required; got %v", err)
	}
	_, err = NewDatabase("mssql", "")
	if err == nil {
		t.Fatal("mssql alias should require DSN")
	}
}

func TestNewDatabase_SQLiteDefaults(t *testing.T) {
	// Use explicit in-memory to avoid polluting the working dir.
	db, err := NewDatabase("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if db == nil {
		t.Fatal("nil db")
	}
	_ = closeDB(db)
}

func TestNewDatabase_DriverCaseInsensitive(t *testing.T) {
	for _, drv := range []string{"SQLite", "SQLITE", "Sqlite"} {
		db, err := NewDatabase(drv, ":memory:")
		if err != nil {
			t.Fatalf("%s: %v", drv, err)
		}
		_ = closeDB(db)
	}
}

func TestAutoMigrateModels_SQLite_AllTablesCreated(t *testing.T) {
	db, err := NewDatabase("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	defer closeDB(db)

	if err := AutoMigrateModels(db, "sqlite"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	for _, table := range []string{"traces", "spans", "logs", "metric_buckets"} {
		var exists int
		db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&exists)
		if exists != 1 {
			t.Fatalf("table %s not created", table)
		}
	}
}

func TestScrubDSN(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		mustContain string // expected token in the output
		mustNotHave string // sensitive token that must be gone
	}{
		{
			name:        "url-form-password",
			in:          "postgres://me:s3cret@h/db",
			mustContain: "REDACTED",
			mustNotHave: "s3cret",
		},
		{
			name:        "url-form-with-port-and-query",
			in:          "postgresql://admin:h@ckme@db.example.com:5432/app?sslmode=require",
			mustContain: "REDACTED",
			mustNotHave: "h@ckme",
		},
		{
			name:        "kv-form-password",
			in:          "host=x user=u password=s3cret sslmode=require",
			mustContain: "password=REDACTED",
			mustNotHave: "s3cret",
		},
		{
			name:        "kv-form-quoted-password",
			in:          `host=x user=u password='s3 cr3t' sslmode=require`,
			mustContain: "password=REDACTED",
			mustNotHave: "s3 cr3t",
		},
		{
			name:        "mixed-case-kv",
			in:          "host=x Password=TopSecret",
			mustContain: "REDACTED",
			mustNotHave: "TopSecret",
		},
		{
			name:        "embedded-in-error-wrap",
			in:          "dial failed: connect host=db user=u password=s3cret: timeout",
			mustContain: "REDACTED",
			mustNotHave: "s3cret",
		},
		{
			name:        "no-password-kv-unchanged",
			in:          "host=x user=u sslmode=require",
			mustContain: "host=x",
			mustNotHave: "REDACTED",
		},
		{
			name:        "empty",
			in:          "",
			mustContain: "",
			mustNotHave: "REDACTED",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := scrubDSN(c.in)
			if c.mustContain != "" && !strings.Contains(got, c.mustContain) {
				t.Fatalf("scrubDSN(%q) = %q; want contains %q", c.in, got, c.mustContain)
			}
			if c.mustNotHave != "" && strings.Contains(got, c.mustNotHave) {
				t.Fatalf("scrubDSN(%q) = %q; leaked %q", c.in, got, c.mustNotHave)
			}
		})
	}
}

func TestAutoMigrateModels_IsIdempotent(t *testing.T) {
	db, err := NewDatabase("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	defer closeDB(db)

	for i := 0; i < 3; i++ {
		if err := AutoMigrateModels(db, "sqlite"); err != nil {
			t.Fatalf("migrate pass %d: %v", i, err)
		}
	}
}

// TestAutoMigrate_CreatesTenantCompositeIndexes asserts that every tenant-scoped
// composite index declared on our models is actually materialised on SQLite.
// Single-column tenant_id indexes were replaced with composites in the storage
// perf pass — this test prevents a silent regression.
func TestAutoMigrate_CreatesTenantCompositeIndexes(t *testing.T) {
	db, err := NewDatabase("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	defer closeDB(db)

	if err := AutoMigrateModels(db, "sqlite"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// (table, expected_index_name) pairs.
	expected := []struct {
		table string
		index string
	}{
		// Trace
		{"traces", "idx_traces_tenant_ts"},
		{"traces", "idx_traces_tenant_service"},
		// Span
		{"spans", "idx_spans_tenant_trace"},
		{"spans", "idx_spans_tenant_service_start"},
		// Log
		{"logs", "idx_logs_tenant_ts"},
		{"logs", "idx_logs_tenant_service"},
		{"logs", "idx_logs_tenant_severity"},
		// MetricBucket
		{"metric_buckets", "idx_metrics_tenant_name_bucket"},
		{"metric_buckets", "idx_metrics_tenant_service_bucket"},
	}

	for _, tc := range expected {
		var count int
		// sqlite_master holds indexes under type='index'.
		err := db.Raw(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND tbl_name=? AND name=?",
			tc.table, tc.index,
		).Scan(&count).Error
		if err != nil {
			t.Fatalf("sqlite_master query for %s.%s: %v", tc.table, tc.index, err)
		}
		if count != 1 {
			t.Errorf("expected composite index %s on table %s (count=%d)", tc.index, tc.table, count)
		}
	}
}
