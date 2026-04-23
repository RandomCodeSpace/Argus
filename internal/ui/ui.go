package ui

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/RandomCodeSpace/otelcontext/internal/graph"
	"github.com/RandomCodeSpace/otelcontext/internal/storage"
	"github.com/RandomCodeSpace/otelcontext/internal/telemetry"
	"github.com/RandomCodeSpace/otelcontext/internal/vectordb"
)

//go:embed templates/*.html static/* dist
var content embed.FS

type Server struct {
	repo       *storage.Repository
	metrics    *telemetry.Metrics
	topo       *graph.Graph
	vidx       *vectordb.Index
	tmpl       *template.Template
	mcpEnabled bool
	mcpPath    string
}

// fmtNum formats an integer-like value with K / M / B suffix.
func fmtNum(v any) string {
	var n float64
	switch val := v.(type) {
	case int:
		n = float64(val)
	case int32:
		n = float64(val)
	case int64:
		n = float64(val)
	case float64:
		n = val
	case float32:
		n = float64(val)
	default:
		return fmt.Sprintf("%v", v)
	}
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.2fB", n/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", n/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", n/1_000)
	default:
		return fmt.Sprintf("%.0f", n)
	}
}

func NewServer(repo *storage.Repository, metrics *telemetry.Metrics, topo *graph.Graph, vidx *vectordb.Index) *Server {
	tmpl := template.New("OtelContext").Funcs(template.FuncMap{
		"text_uppercase": strings.ToUpper,
		"text_lowercase": strings.ToLower,
		"fmt_num":        fmtNum,
	})
	tmpl = template.Must(tmpl.ParseFS(content, "templates/*.html"))

	return &Server{
		repo:    repo,
		metrics: metrics,
		topo:    topo,
		vidx:    vidx,
		tmpl:    tmpl,
		mcpPath: "/mcp",
	}
}

// SetMCPConfig configures MCP metadata shown in the UI.
func (s *Server) SetMCPConfig(enabled bool, path string) {
	s.mcpEnabled = enabled
	if path != "" {
		s.mcpPath = path
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) error {
	mux.Handle("/static/", http.FileServer(http.FS(content)))

	// Serve React SPA from dist/ for all non-API paths.
	// API routes are registered before this is called, so they take priority.
	distFS, err := fs.Sub(content, "dist")
	if err != nil {
		return fmt.Errorf("ui: failed to create dist sub-fs: %w", err)
	}
	fileServer := http.FileServer(http.FS(distFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try the file as-is; if not found, fall back to index.html (SPA routing).
		f, openErr := distFS.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if openErr == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback — let the React router handle the path.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})

	return nil
}
