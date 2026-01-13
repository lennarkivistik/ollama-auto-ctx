// Package api provides the versioned REST API for telemetry data.
// All endpoints are under /autoctx/api/v1/ to avoid collision with Ollama.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"ollama-auto-ctx/internal/config"
	"ollama-auto-ctx/internal/storage"
)

const (
	// APIPrefix is the base path for all API endpoints.
	APIPrefix = "/autoctx/api/v1"

	// Cache duration for overview responses (prevents refresh storms).
	overviewCacheDuration = 2 * time.Second
)

// Server handles API requests for telemetry data.
type Server struct {
	store  storage.Store
	cfg    config.Config
	logger *slog.Logger

	// Overview cache to prevent refresh storms
	overviewCache     map[string]*cachedOverview
	overviewCacheMu   sync.RWMutex
}

type cachedOverview struct {
	data      *OverviewResponse
	expiresAt time.Time
}

// NewServer creates a new API server.
func NewServer(store storage.Store, cfg config.Config, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		store:         store,
		cfg:           cfg,
		logger:        logger,
		overviewCache: make(map[string]*cachedOverview),
	}
}

// ServeHTTP handles API requests.
// It expects paths starting with /autoctx/api/v1/.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract path after prefix
	path := strings.TrimPrefix(r.URL.Path, APIPrefix)
	if path == r.URL.Path {
		// Path doesn't start with our prefix
		http.NotFound(w, r)
		return
	}

	// Route to handlers
	switch {
	case path == "/overview" && r.Method == http.MethodGet:
		s.handleOverview(w, r)
	case path == "/requests" && r.Method == http.MethodGet:
		s.handleListRequests(w, r)
	case strings.HasPrefix(path, "/requests/") && r.Method == http.MethodGet:
		id := strings.TrimPrefix(path, "/requests/")
		s.handleGetRequest(w, r, id)
	case path == "/models" && r.Method == http.MethodGet:
		s.handleListModels(w, r)
	case strings.HasPrefix(path, "/models/") && strings.HasSuffix(path, "/series") && r.Method == http.MethodGet:
		model := strings.TrimPrefix(path, "/models/")
		model = strings.TrimSuffix(model, "/series")
		s.handleModelSeries(w, r, model)
	case path == "/config" && r.Method == http.MethodGet:
		s.handleConfig(w, r)
	default:
		http.NotFound(w, r)
	}
}

// Handles handles a request, returning true if it was an API request.
func (s *Server) Handles(path string) bool {
	return strings.HasPrefix(path, APIPrefix)
}

// Helper functions

func (s *Server) writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("failed to encode JSON response", "err", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func parseWindow(r *http.Request) time.Duration {
	w := r.URL.Query().Get("window")
	switch w {
	case "1h":
		return time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "24h", "":
		return 24 * time.Hour
	default:
		// Try to parse as duration
		if d, err := time.ParseDuration(w); err == nil {
			return d
		}
		return 24 * time.Hour
	}
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return def
		}
	}
	return n
}
