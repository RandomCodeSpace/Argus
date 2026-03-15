package api

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

// handleSearchColdArchive handles GET /api/archive/search
// Query params: type=logs|traces|metrics, start=RFC3339, end=RFC3339, q=text
func (s *Server) handleSearchColdArchive(w http.ResponseWriter, r *http.Request) {
	dataType := r.URL.Query().Get("type")
	if dataType == "" {
		dataType = "logs"
	}
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	query := strings.ToLower(r.URL.Query().Get("q"))

	end := time.Now()
	start := end.Add(-7 * 24 * time.Hour)
	if t, err := time.Parse(time.RFC3339, startStr); err == nil {
		start = t
	}
	if t, err := time.Parse(time.RFC3339, endStr); err == nil {
		end = t
	}

	coldPath := s.coldStoragePath()
	if coldPath == "" {
		http.Error(w, "cold storage not configured", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	enc := json.NewEncoder(w)
	count := 0

	// Walk day directories in the date range.
	for d := start.UTC().Truncate(24 * time.Hour); !d.After(end.UTC()); d = d.Add(24 * time.Hour) {
		dir := filepath.Join(coldPath, d.Format("2006"), d.Format("01"), d.Format("02"))
		filename := filepath.Join(dir, dataType+".jsonl.zst")

		records, err := readZSTJSONL(filename, query)
		if err != nil {
			continue // day not archived yet or file missing
		}
		for _, rec := range records {
			enc.Encode(map[string]any{"source": "cold", "date": d.Format("2006-01-02"), "data": rec})
			count++
			if count >= 1000 {
				return
			}
		}
	}
}

// coldStoragePath returns the cold storage base path from config via the repo.
// We store it in the Server via a simple string field set at init.
func (s *Server) coldStoragePath() string {
	return s.coldPath
}

// readZSTJSONL decompresses a .jsonl.zst file and returns matching raw JSON objects.
// If query is empty all records are returned (up to 1000).
func readZSTJSONL(path, query string) ([]json.RawMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reader io.Reader

	switch {
	case strings.HasSuffix(path, ".zst"):
		dec, err := zstd.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer dec.Close()
		reader = dec
	case strings.HasSuffix(path, ".gz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	default:
		reader = f
	}

	var results []json.RawMessage
	dec := json.NewDecoder(reader)
	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			break
		}
		if query == "" || strings.Contains(strings.ToLower(string(raw)), query) {
			results = append(results, raw)
			if len(results) >= 1000 {
				break
			}
		}
	}
	return results, nil
}
