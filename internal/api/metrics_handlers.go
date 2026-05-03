package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/RandomCodeSpace/otelcontext/internal/api/views"
	"github.com/RandomCodeSpace/otelcontext/internal/httpconst"
)

// handleGetTrafficMetrics handles GET /api/metrics/traffic
func (s *Server) handleGetTrafficMetrics(w http.ResponseWriter, r *http.Request) {
	// Default to last 30 minutes if not specified
	end := time.Now()
	start := end.Add(-30 * time.Minute)

	if startStr := r.URL.Query().Get("start"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			start = t
		}
	}
	if endStr := r.URL.Query().Get("end"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			end = t
		}
	}

	serviceNames := r.URL.Query()["service_name"]

	points, err := s.repo.GetTrafficMetrics(r.Context(), start, end, serviceNames)
	if err != nil {
		slog.Error("Failed to get traffic metrics", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	_ = json.NewEncoder(w).Encode(points)
}

// handleGetLatencyHeatmap handles GET /api/metrics/latency_heatmap
func (s *Server) handleGetLatencyHeatmap(w http.ResponseWriter, r *http.Request) {
	end := time.Now()
	start := end.Add(-30 * time.Minute)

	if startStr := r.URL.Query().Get("start"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			start = t
		}
	}
	if endStr := r.URL.Query().Get("end"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			end = t
		}
	}

	serviceNames := r.URL.Query()["service_name"]

	points, err := s.repo.GetLatencyHeatmap(r.Context(), start, end, serviceNames)
	if err != nil {
		slog.Error("Failed to get latency heatmap", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	_ = json.NewEncoder(w).Encode(points)
}

// handleGetDashboardStats handles GET /api/metrics/dashboard
func (s *Server) handleGetDashboardStats(w http.ResponseWriter, r *http.Request) {
	// Default to last 30 minutes if not specified
	end := time.Now()
	start := end.Add(-30 * time.Minute)

	if startStr := r.URL.Query().Get("start"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			start = t
		}
	}
	if endStr := r.URL.Query().Get("end"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			end = t
		}
	}

	serviceNames := r.URL.Query()["service_name"]

	stats, err := s.repo.GetDashboardStats(r.Context(), start, end, serviceNames)
	if err != nil {
		slog.Error("Failed to get dashboard stats", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	_ = json.NewEncoder(w).Encode(views.DashboardStatsFromModel(stats))
}

// handleGetServiceMapMetrics handles GET /api/metrics/service-map
func (s *Server) handleGetServiceMapMetrics(w http.ResponseWriter, r *http.Request) {
	end := time.Now()
	start := end.Add(-30 * time.Minute)

	if startStr := r.URL.Query().Get("start"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			start = t
		}
	}
	if endStr := r.URL.Query().Get("end"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			end = t
		}
	}

	metrics, err := s.repo.GetServiceMapMetrics(r.Context(), start, end)
	if err != nil {
		slog.Error("Failed to get service map metrics", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	_ = json.NewEncoder(w).Encode(views.ServiceMapMetricsFromModel(metrics))
}

// handleGetMetricBuckets handles GET /api/metrics
func (s *Server) handleGetMetricBuckets(w http.ResponseWriter, r *http.Request) {
	start, end, err := parseTimeRange(r)
	if err != nil {
		http.Error(w, "invalid time range", http.StatusBadRequest)
		return
	}

	name := r.URL.Query().Get("name")
	serviceName := r.URL.Query().Get("service_name")

	// name is required for bucket queries
	if name == "" {
		http.Error(w, "metric name is required", http.StatusBadRequest)
		return
	}

	buckets, err := s.repo.GetMetricBuckets(r.Context(), start, end, serviceName, name)
	if err != nil {
		slog.Error("Failed to get metric buckets", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	_ = json.NewEncoder(w).Encode(views.MetricBucketsFromModels(buckets))
}

// handleGetMetricNames handles GET /api/metadata/metrics
func (s *Server) handleGetMetricNames(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("service_name")

	names, err := s.repo.GetMetricNames(r.Context(), serviceName)
	if err != nil {
		slog.Error("Failed to get metric names", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	_ = json.NewEncoder(w).Encode(names)
}

// handleGetServices returns the list of services the caller's tenant has
// emitted any span for. Read from the in-memory GraphRAG ServiceStore so
// the dropdown matches /api/system/graph exactly — and so a service that
// only appears as a downstream callee (e.g. shipping-service deep in a
// fan-out) isn't silently dropped because some other span won the
// trace_id-uniqueness race for the legacy `traces` table query.
//
// Cold-start (first ~60s after restart, before the GraphRAG refresh loop
// rebuilds from DB) returns an empty list, which is correct: nothing has
// been ingested yet that the dropdown could meaningfully display.
func (s *Server) handleGetServices(w http.ResponseWriter, r *http.Request) {
	var services []string
	if s.graphRAG != nil {
		services = s.graphRAG.ServiceNames(r.Context())
	}
	if services == nil {
		services = []string{}
	}
	w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	_ = json.NewEncoder(w).Encode(services)
}
