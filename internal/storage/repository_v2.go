package storage

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// TrafficPoint represents a data point for the traffic chart.
type TrafficPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	Count      int64     `json:"count"`
	ErrorCount int64     `json:"error_count"`
}

// LatencyPoint represents a data point for the latency heatmap.
type LatencyPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Duration  int64     `json:"duration"` // Microseconds
}

// ServiceError represents error counts per service.
type ServiceError struct {
	ServiceName string  `json:"service_name"`
	ErrorCount  int64   `json:"error_count"`
	TotalCount  int64   `json:"total_count"`
	ErrorRate   float64 `json:"error_rate"`
}

// DashboardStats represents aggregated metrics for the dashboard.
type DashboardStats struct {
	TotalTraces        int64          `json:"total_traces"`
	TotalLogs          int64          `json:"total_logs"`
	TotalErrors        int64          `json:"total_errors"`
	AvgLatencyMs       float64        `json:"avg_latency_ms"`
	ErrorRate          float64        `json:"error_rate"`
	ActiveServices     int64          `json:"active_services"`
	P99Latency         int64          `json:"p99_latency"`
	TopFailingServices []ServiceError `json:"top_failing_services"`
}

// LogFilter defines criteria for searching logs.
type LogFilter struct {
	ServiceName string
	Severity    string
	Search      string // Full-text search
	StartTime   time.Time
	EndTime     time.Time
	Limit       int
	Offset      int
}

// GetTrafficMetrics returns request counts bucketed by minute, including error counts.
func (r *Repository) GetTrafficMetrics(start, end time.Time, serviceNames []string) ([]TrafficPoint, error) {
	var points []TrafficPoint

	// Fetch timestamps + status for traffic + error breakdown
	type traceRow struct {
		Timestamp time.Time
		Status    string
	}
	var rows []traceRow

	query := r.db.Model(&Trace{}).
		Select("timestamp, status").
		Where("timestamp BETWEEN ? AND ?", start, end)

	if len(serviceNames) > 0 {
		query = query.Where("service_name IN ?", serviceNames)
	}

	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}

	type bucket struct {
		count      int64
		errorCount int64
	}
	buckets := make(map[int64]*bucket)
	for _, r := range rows {
		ts := r.Timestamp.Truncate(time.Minute).Unix()
		b, ok := buckets[ts]
		if !ok {
			b = &bucket{}
			buckets[ts] = b
		}
		b.count++
		if strings.Contains(r.Status, "ERROR") {
			b.errorCount++
		}
	}

	for ts, b := range buckets {
		points = append(points, TrafficPoint{
			Timestamp:  time.Unix(ts, 0),
			Count:      b.count,
			ErrorCount: b.errorCount,
		})
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})

	return points, nil
}

// GetLatencyHeatmap returns trace duration and timestamps for heatmap rendering.
func (r *Repository) GetLatencyHeatmap(start, end time.Time, serviceNames []string) ([]LatencyPoint, error) {
	var points []LatencyPoint
	query := r.db.Model(&Trace{}).
		Select("timestamp, duration").
		Where("timestamp BETWEEN ? AND ?", start, end)

	if len(serviceNames) > 0 {
		query = query.Where("service_name IN ?", serviceNames)
	}

	err := query.Order("timestamp DESC").
		Limit(2000).
		Find(&points).Error

	if err != nil {
		return nil, err
	}
	return points, nil
}

// GetDashboardStats calculates high-level metrics for the dashboard.
func (r *Repository) GetDashboardStats(start, end time.Time, serviceNames []string) (*DashboardStats, error) {
	var stats DashboardStats

	baseQuery := r.db.Model(&Trace{}).Where("timestamp BETWEEN ? AND ?", start, end)
	if len(serviceNames) > 0 {
		baseQuery = baseQuery.Where("service_name IN ?", serviceNames)
	}

	// 1. Total Traces
	if err := baseQuery.Session(&gorm.Session{}).Count(&stats.TotalTraces).Error; err != nil {
		return nil, fmt.Errorf("failed to count traces: %w", err)
	}

	// 2. Total Logs
	logQuery := r.db.Model(&Log{}).Where("timestamp BETWEEN ? AND ?", start, end)
	if len(serviceNames) > 0 {
		logQuery = logQuery.Where("service_name IN ?", serviceNames)
	}
	if err := logQuery.Count(&stats.TotalLogs).Error; err != nil {
		return nil, fmt.Errorf("failed to count logs: %w", err)
	}

	// 3. Total Errors (traces with error status)
	if err := baseQuery.Session(&gorm.Session{}).
		Where("status LIKE ?", "%ERROR%").
		Count(&stats.TotalErrors).Error; err != nil {
		return nil, fmt.Errorf("failed to count error traces: %w", err)
	}

	if stats.TotalTraces > 0 {
		stats.ErrorRate = (float64(stats.TotalErrors) / float64(stats.TotalTraces)) * 100
	}

	// 4. Average Latency (microseconds → milliseconds)
	type avgResult struct {
		Avg float64
	}
	var avg avgResult
	if err := baseQuery.Session(&gorm.Session{}).
		Select("COALESCE(AVG(duration), 0) as avg").
		Scan(&avg).Error; err != nil {
		log.Printf("⚠️ Failed to compute avg latency: %v", err)
	} else {
		stats.AvgLatencyMs = avg.Avg / 1000.0 // microseconds → ms
	}

	// 5. Active Services
	if err := baseQuery.Session(&gorm.Session{}).
		Distinct("service_name").
		Count(&stats.ActiveServices).Error; err != nil {
		return nil, fmt.Errorf("failed to count active services: %w", err)
	}

	// 6. P99 Latency
	var durations []int64
	if err := baseQuery.Session(&gorm.Session{}).
		Select("duration").
		Find(&durations).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch durations: %w", err)
	}

	if len(durations) > 0 {
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		p99Index := int(math.Ceil(float64(len(durations))*0.99)) - 1
		if p99Index >= len(durations) {
			p99Index = len(durations) - 1
		}
		if p99Index < 0 {
			p99Index = 0
		}
		stats.P99Latency = durations[p99Index]
	}

	// 7. Top Failing Services (with error rate)
	type svcCount struct {
		ServiceName string
		ErrorCount  int64
		TotalCount  int64
	}
	var svcCounts []svcCount
	if err := baseQuery.Session(&gorm.Session{}).
		Select("service_name, COUNT(*) as total_count, SUM(CASE WHEN status LIKE '%ERROR%' THEN 1 ELSE 0 END) as error_count").
		Group("service_name").
		Having("error_count > 0").
		Order("error_count DESC").
		Limit(5).
		Scan(&svcCounts).Error; err != nil {
		log.Printf("⚠️ Failed to fetch top failing services: %v", err)
	} else {
		for _, sc := range svcCounts {
			rate := 0.0
			if sc.TotalCount > 0 {
				rate = float64(sc.ErrorCount) / float64(sc.TotalCount)
			}
			stats.TopFailingServices = append(stats.TopFailingServices, ServiceError{
				ServiceName: sc.ServiceName,
				ErrorCount:  sc.ErrorCount,
				TotalCount:  sc.TotalCount,
				ErrorRate:   rate,
			})
		}
	}

	return &stats, nil
}

// TracesResponse represents the response for the traces endpoint with pagination
type TracesResponse struct {
	Traces []Trace `json:"traces"`
	Total  int64   `json:"total"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
}

// GetTracesFiltered retrieves traces with filtering and pagination
func (r *Repository) GetTracesFiltered(start, end time.Time, serviceNames []string, status, search string, limit, offset int, sortBy, orderBy string) (*TracesResponse, error) {
	var traces []Trace
	var total int64

	// Build base query
	query := r.db.Model(&Trace{})

	// Apply filters
	if !start.IsZero() && !end.IsZero() {
		query = query.Where("timestamp BETWEEN ? AND ?", start, end)
	}

	if len(serviceNames) > 0 {
		query = query.Where("service_name IN ?", serviceNames)
	}

	if status != "" {
		query = query.Where("status LIKE ?", "%"+status+"%")
	}

	if search != "" {
		query = query.Where("trace_id LIKE ?", "%"+search+"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count traces: %w", err)
	}

	// Apply Sorting
	orderClause := "timestamp DESC" // Default
	if sortBy != "" {
		direction := "ASC"
		if orderBy == "desc" {
			direction = "DESC"
		}
		// Whitelist fields to prevent SQL injection
		validSorts := map[string]string{
			"timestamp":    "timestamp",
			"duration":     "duration",
			"service_name": "service_name",
			"status":       "status",
			"trace_id":     "trace_id",
		}
		if field, ok := validSorts[sortBy]; ok {
			orderClause = fmt.Sprintf("%s %s", field, direction)
		}
	}

	// Get paginated results with spans preloaded
	if err := query.
		Preload("Spans").
		Order(orderClause).
		Limit(limit).
		Offset(offset).
		Find(&traces).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch traces: %w", err)
	}

	// Populate virtual fields for frontend
	for i := range traces {
		traces[i].SpanCount = len(traces[i].Spans)
		traces[i].DurationMs = float64(traces[i].Duration) / 1000.0
		if traces[i].SpanCount > 0 {
			traces[i].Operation = traces[i].Spans[0].OperationName
		} else {
			traces[i].Operation = "Unknown"
		}
	}

	return &TracesResponse{
		Traces: traces,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

// GetLogsV2 performs advanced filtering and search on logs.
func (r *Repository) GetLogsV2(filter LogFilter) ([]Log, int64, error) {
	var logs []Log
	var total int64

	query := r.db.Model(&Log{})

	if filter.ServiceName != "" {
		query = query.Where("service_name = ?", filter.ServiceName)
	}
	if filter.Severity != "" {
		query = query.Where("severity = ?", filter.Severity)
	}
	if !filter.StartTime.IsZero() {
		query = query.Where("timestamp >= ?", filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		query = query.Where("timestamp <= ?", filter.EndTime)
	}
	if filter.Search != "" {
		search := "%" + filter.Search + "%"
		query = query.Where("body LIKE ? OR trace_id LIKE ?", search, search)
	}

	// Count total for pagination
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch page
	if err := query.Order("timestamp desc").
		Limit(filter.Limit).
		Offset(filter.Offset).
		Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// GetLogContext returns logs surrounding a specific timestamp (+/- 1 minute).
func (r *Repository) GetLogContext(targetTime time.Time) ([]Log, error) {
	start := targetTime.Add(-1 * time.Minute)
	end := targetTime.Add(1 * time.Minute)

	var logs []Log
	if err := r.db.Where("timestamp BETWEEN ? AND ?", start, end).
		Order("timestamp asc").
		Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// ServiceMapNode represents a single service node on the service map.
type ServiceMapNode struct {
	Name         string  `json:"name"`
	TotalTraces  int64   `json:"total_traces"`
	ErrorCount   int64   `json:"error_count"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// ServiceMapEdge represents a connection between two services.
type ServiceMapEdge struct {
	Source       string  `json:"source"`
	Target       string  `json:"target"`
	CallCount    int64   `json:"call_count"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	ErrorRate    float64 `json:"error_rate"`
}

// ServiceMapMetrics holds the complete service topology with metrics.
type ServiceMapMetrics struct {
	Nodes []ServiceMapNode `json:"nodes"`
	Edges []ServiceMapEdge `json:"edges"`
}

// GetServiceMapMetrics computes per-service and per-edge metrics from traces and spans.
func (r *Repository) GetServiceMapMetrics(start, end time.Time) (*ServiceMapMetrics, error) {
	// 1. Per-service node metrics from traces
	type nodeRow struct {
		ServiceName string
		Total       int64
		Errors      int64
		AvgDuration float64
	}
	var nodeRows []nodeRow

	nodeQuery := r.db.Model(&Trace{}).
		Select("service_name, COUNT(*) as total, SUM(CASE WHEN status LIKE '%ERROR%' THEN 1 ELSE 0 END) as errors, AVG(duration) as avg_duration").
		Group("service_name")

	if !start.IsZero() && !end.IsZero() {
		nodeQuery = nodeQuery.Where("timestamp BETWEEN ? AND ?", start, end)
	}

	if err := nodeQuery.Find(&nodeRows).Error; err != nil {
		return nil, fmt.Errorf("failed to get service map nodes: %w", err)
	}

	nodes := make([]ServiceMapNode, 0, len(nodeRows))
	for _, nr := range nodeRows {
		if nr.ServiceName == "" {
			continue
		}
		nodes = append(nodes, ServiceMapNode{
			Name:         nr.ServiceName,
			TotalTraces:  nr.Total,
			ErrorCount:   nr.Errors,
			AvgLatencyMs: math.Round(nr.AvgDuration/1000*100) / 100, // µs → ms
		})
	}

	// 2. Per-edge metrics: find traces that span multiple services via spans table
	type spanRow struct {
		TraceID       string
		OperationName string
		Duration      int64
		Status        string
	}

	// Get all spans in the time range, grouped by trace
	var spans []Span
	spanQuery := r.db.Model(&Span{})
	if !start.IsZero() && !end.IsZero() {
		// Join with traces to filter by time range
		spanQuery = spanQuery.Joins("JOIN traces ON spans.trace_id = traces.trace_id").
			Where("traces.timestamp BETWEEN ? AND ?", start, end)
	}
	if err := spanQuery.Find(&spans).Error; err != nil {
		return nil, fmt.Errorf("failed to get spans for service map: %w", err)
	}

	// Build trace → services mapping from traces (not spans, since spans don't have service_name)
	type traceInfo struct {
		TraceID     string
		ServiceName string
		Status      string
		Duration    int64
	}
	var traceInfos []traceInfo
	tiQuery := r.db.Model(&Trace{}).Select("trace_id, service_name, status, duration")
	if !start.IsZero() && !end.IsZero() {
		tiQuery = tiQuery.Where("timestamp BETWEEN ? AND ?", start, end)
	}
	if err := tiQuery.Find(&traceInfos).Error; err != nil {
		return nil, fmt.Errorf("failed to get trace infos: %w", err)
	}

	// Group by trace_id to find multi-service traces
	traceServiceMap := make(map[string]map[string]struct {
		count  int64
		errors int64
		totalD int64
	})
	for _, ti := range traceInfos {
		if ti.ServiceName == "" {
			continue
		}
		if _, ok := traceServiceMap[ti.TraceID]; !ok {
			traceServiceMap[ti.TraceID] = make(map[string]struct {
				count  int64
				errors int64
				totalD int64
			})
		}
		entry := traceServiceMap[ti.TraceID][ti.ServiceName]
		entry.count++
		if strings.Contains(ti.Status, "ERROR") {
			entry.errors++
		}
		entry.totalD += ti.Duration
		traceServiceMap[ti.TraceID][ti.ServiceName] = entry
	}

	// Derive edges from traces that touch multiple services
	type edgeKey struct{ source, target string }
	edgeAgg := make(map[edgeKey]struct {
		calls   int64
		errors  int64
		totalMs float64
	})

	for _, services := range traceServiceMap {
		svcNames := make([]string, 0, len(services))
		for name := range services {
			svcNames = append(svcNames, name)
		}
		sort.Strings(svcNames)

		for i := 0; i < len(svcNames); i++ {
			for j := i + 1; j < len(svcNames); j++ {
				key := edgeKey{source: svcNames[i], target: svcNames[j]}
				entry := edgeAgg[key]
				entry.calls++
				// Use the average duration of both services for this edge
				si := services[svcNames[i]]
				sj := services[svcNames[j]]
				avgD := float64(si.totalD+sj.totalD) / float64(si.count+sj.count) / 1000.0 // µs → ms
				entry.totalMs += avgD
				if si.errors > 0 || sj.errors > 0 {
					entry.errors++
				}
				edgeAgg[key] = entry
			}
		}
	}

	edges := make([]ServiceMapEdge, 0, len(edgeAgg))
	// Compute time range duration in minutes for calls/min
	rangeMins := end.Sub(start).Minutes()
	if rangeMins < 1 {
		rangeMins = 1
	}

	for key, agg := range edgeAgg {
		errRate := float64(0)
		if agg.calls > 0 {
			errRate = math.Round(float64(agg.errors)/float64(agg.calls)*1000) / 1000
		}
		edges = append(edges, ServiceMapEdge{
			Source:       key.source,
			Target:       key.target,
			CallCount:    agg.calls,
			AvgLatencyMs: math.Round(agg.totalMs/float64(agg.calls)*100) / 100,
			ErrorRate:    errRate,
		})
	}

	return &ServiceMapMetrics{
		Nodes: nodes,
		Edges: edges,
	}, nil
}
