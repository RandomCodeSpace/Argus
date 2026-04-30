package storage

// Shared SQL fragments used by repository methods across log_repo, trace_repo,
// metrics_repo, repository.go and partitions.go. Centralised here so the same
// fragment isn't duplicated across files (Sonar S1192).
//
// These are not exported — they're internal building blocks for GORM `.Where()`
// calls and `.Order()` clauses, not part of any public contract.
const (
	sqlWhereTenantID          = "tenant_id = ?"
	sqlWhereSeverity          = "severity = ?"
	sqlWhereTimestampGTE      = "timestamp >= ?"
	sqlWhereTimestampLTE      = "timestamp <= ?"
	sqlOrderTimestampDesc     = "timestamp desc"
	sqlWhereTenantTimeBetween = "tenant_id = ? AND timestamp BETWEEN ? AND ?"
	sqlWhereServiceIn         = "service_name IN ?"

	// cacheKeyTelemetryStart is the in-memory cache key for the
	// telemetry-start timestamp (used by the dashboard "uptime" tile).
	cacheKeyTelemetryStart = "telemetry:start_time"

	// timeFormatPGUTC is the Postgres-native timestamptz format string used
	// when materialising partition boundary literals.
	timeFormatPGUTC = "2006-01-02 15:04:05+00"
)
