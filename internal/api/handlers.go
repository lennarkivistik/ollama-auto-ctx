package api

import (
	"net/http"
	"time"

	"ollama-auto-ctx/internal/storage"
)

// OverviewResponse contains summary statistics and time series data.
type OverviewResponse struct {
	Summary SummaryData `json:"summary"`
	Series  SeriesData  `json:"series"`
}

// SummaryData contains aggregate statistics.
type SummaryData struct {
	TotalRequests int     `json:"total_requests"`
	SuccessRate   float64 `json:"success_rate"`
	AvgDurationMs int     `json:"avg_duration_ms"`
	P95DurationMs int     `json:"p95_duration_ms"`
	TotalBytes    int64   `json:"total_bytes"`
	TotalTokens   int     `json:"total_tokens"`
	Retries       int     `json:"retries"`
	Timeouts      int     `json:"timeouts"`
	Loops         int     `json:"loops"`
	InFlight      int     `json:"in_flight"`
}

// SeriesData contains time-binned chart data.
type SeriesData struct {
	DurationP95    []storage.DataPoint `json:"duration_p95"`
	RequestCount   []storage.DataPoint `json:"req_count"`
	GenTokPerS     []storage.DataPoint `json:"gen_tok_per_s"`
	CtxUtilization []storage.DataPoint `json:"ctx_utilization"`
}

// handleOverview returns summary statistics and time series.
// GET /autoctx/api/v1/overview?window=1h|24h|7d
func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		s.writeError(w, http.StatusServiceUnavailable, "storage not available")
		return
	}

	window := parseWindow(r)
	cacheKey := window.String()

	// Check cache
	s.overviewCacheMu.RLock()
	if cached, ok := s.overviewCache[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		s.overviewCacheMu.RUnlock()
		s.writeJSON(w, cached.data)
		return
	}
	s.overviewCacheMu.RUnlock()

	// Fetch fresh data
	overview, err := s.store.Overview(window)
	if err != nil {
		s.logger.Error("failed to get overview", "err", err)
		s.writeError(w, http.StatusInternalServerError, "failed to get overview")
		return
	}

	inFlight, _ := s.store.InFlightCount()

	// Fetch series data
	durationSeries, _ := s.store.Series(storage.SeriesOptions{Window: window, Metric: "duration_p95"})
	reqCountSeries, _ := s.store.Series(storage.SeriesOptions{Window: window, Metric: "req_count"})
	genTokSeries, _ := s.store.Series(storage.SeriesOptions{Window: window, Metric: "gen_tok_per_s"})
	ctxUtilSeries, _ := s.store.Series(storage.SeriesOptions{Window: window, Metric: "ctx_utilization"})

	resp := &OverviewResponse{
		Summary: SummaryData{
			TotalRequests: overview.TotalRequests,
			SuccessRate:   overview.SuccessRate,
			AvgDurationMs: overview.AvgDurationMs,
			P95DurationMs: overview.P95DurationMs,
			TotalBytes:    overview.TotalBytes,
			TotalTokens:   overview.TotalTokens,
			Retries:       overview.Retries,
			Timeouts:      overview.Timeouts,
			Loops:         overview.Loops,
			InFlight:      inFlight,
		},
		Series: SeriesData{
			DurationP95:    durationSeries,
			RequestCount:   reqCountSeries,
			GenTokPerS:     genTokSeries,
			CtxUtilization: ctxUtilSeries,
		},
	}

	// Update cache
	s.overviewCacheMu.Lock()
	s.overviewCache[cacheKey] = &cachedOverview{
		data:      resp,
		expiresAt: time.Now().Add(overviewCacheDuration),
	}
	s.overviewCacheMu.Unlock()

	s.writeJSON(w, resp)
}

// RequestListItem is a summary of a request for list views.
type RequestListItem struct {
	ID            string  `json:"id"`
	Timestamp     int64   `json:"ts"`
	Model         string  `json:"model"`
	Endpoint      string  `json:"endpoint"`
	DurationMs    int     `json:"duration_ms"`
	ClientOutBytes int64  `json:"bytes"`
	Status        string  `json:"status"`
	Reason        string  `json:"reason,omitempty"`
}

// RequestListResponse contains paginated request list.
type RequestListResponse struct {
	Requests []RequestListItem `json:"requests"`
	Total    int               `json:"total"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
}

// handleListRequests returns a paginated list of requests.
// GET /autoctx/api/v1/requests?limit=50&offset=0&status=&model=&reason=&window=24h
func (s *Server) handleListRequests(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		s.writeError(w, http.StatusServiceUnavailable, "storage not available")
		return
	}

	q := r.URL.Query()
	limit := parseInt(q.Get("limit"), 50)
	offset := parseInt(q.Get("offset"), 0)
	window := parseWindow(r)

	opts := storage.ListOptions{
		Limit:  limit,
		Offset: offset,
		Window: window,
	}

	if status := q.Get("status"); status != "" {
		s := storage.Status(status)
		opts.Status = &s
	}
	if model := q.Get("model"); model != "" {
		opts.Model = model
	}
	if reason := q.Get("reason"); reason != "" {
		r := storage.Reason(reason)
		opts.Reason = &r
	}

	requests, err := s.store.List(opts)
	if err != nil {
		s.logger.Error("failed to list requests", "err", err)
		s.writeError(w, http.StatusInternalServerError, "failed to list requests")
		return
	}

	// Convert to list items
	items := make([]RequestListItem, len(requests))
	for i, req := range requests {
		items[i] = RequestListItem{
			ID:            req.ID,
			Timestamp:     req.TSStart,
			Model:         req.Model,
			Endpoint:      req.Endpoint,
			DurationMs:    req.DurationMs,
			ClientOutBytes: req.ClientOutBytes,
			Status:        string(req.Status),
			Reason:        string(req.Reason),
		}
	}

	s.writeJSON(w, RequestListResponse{
		Requests: items,
		Total:    len(items), // TODO: implement total count query
		Limit:    limit,
		Offset:   offset,
	})
}

// RequestDetailResponse contains full details for a single request.
type RequestDetailResponse struct {
	// Identity
	ID       string `json:"id"`
	TSStart  int64  `json:"ts_start"`
	TSEnd    *int64 `json:"ts_end"`
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
	Model    string `json:"model"`
	Endpoint string `json:"endpoint"`

	// Request shape
	Request RequestShape `json:"request"`

	// AutoCTX decision
	AutoCTX AutoCTXData `json:"autoctx"`

	// Ollama response
	Ollama OllamaData `json:"ollama"`

	// Response summary
	Response ResponseData `json:"response"`
}

// RequestShape contains request structure metadata.
type RequestShape struct {
	MessagesCount   int    `json:"messages_count"`
	SystemChars     int    `json:"system_chars"`
	UserChars       int    `json:"user_chars"`
	AssistantChars  int    `json:"assistant_chars"`
	ToolsCount      int    `json:"tools_count"`
	ToolChoice      string `json:"tool_choice,omitempty"`
	StreamRequested bool   `json:"stream_requested"`
	ClientInBytes   int64  `json:"client_in_bytes"`
}

// AutoCTXData contains context sizing decisions.
type AutoCTXData struct {
	CtxEst       int `json:"ctx_est"`
	CtxSelected  int `json:"ctx_selected"`
	CtxBucket    int `json:"ctx_bucket"`
	OutputBudget int `json:"output_budget"`
}

// OllamaData contains upstream response data.
type OllamaData struct {
	PromptTokens         int   `json:"prompt_tokens"`
	CompletionTokens     int   `json:"completion_tokens"`
	UpstreamInBytes      int64 `json:"upstream_in_bytes"`
	UpstreamOutBytes     int64 `json:"upstream_out_bytes"`
	UpstreamTotalMs      int   `json:"upstream_total_ms"`
	UpstreamLoadMs       int   `json:"upstream_load_ms"`
	UpstreamPromptEvalMs int   `json:"upstream_prompt_eval_ms"`
	UpstreamEvalMs       int   `json:"upstream_eval_ms"`
	HTTPStatus           int   `json:"http_status,omitempty"`
}

// ResponseData contains final response summary.
type ResponseData struct {
	DurationMs     int    `json:"duration_ms"`
	TTFBMs         int    `json:"ttfb_ms"`
	ClientOutBytes int64  `json:"client_out_bytes"`
	RetryCount     int    `json:"retry_count"`
	ErrorClass     string `json:"error_class,omitempty"`
}

// handleGetRequest returns full details for a single request.
// GET /autoctx/api/v1/requests/{id}
func (s *Server) handleGetRequest(w http.ResponseWriter, r *http.Request, id string) {
	if s.store == nil {
		s.writeError(w, http.StatusServiceUnavailable, "storage not available")
		return
	}

	req, err := s.store.GetByID(id)
	if err != nil {
		s.logger.Error("failed to get request", "err", err, "id", id)
		s.writeError(w, http.StatusInternalServerError, "failed to get request")
		return
	}
	if req == nil {
		s.writeError(w, http.StatusNotFound, "request not found")
		return
	}

	resp := RequestDetailResponse{
		ID:       req.ID,
		TSStart:  req.TSStart,
		TSEnd:    req.TSEnd,
		Status:   string(req.Status),
		Reason:   string(req.Reason),
		Model:    req.Model,
		Endpoint: req.Endpoint,
		Request: RequestShape{
			MessagesCount:   req.MessagesCount,
			SystemChars:     req.SystemChars,
			UserChars:       req.UserChars,
			AssistantChars:  req.AssistantChars,
			ToolsCount:      req.ToolsCount,
			ToolChoice:      req.ToolChoice,
			StreamRequested: req.StreamRequested,
			ClientInBytes:   req.ClientInBytes,
		},
		AutoCTX: AutoCTXData{
			CtxEst:       req.CtxEst,
			CtxSelected:  req.CtxSelected,
			CtxBucket:    req.CtxBucket,
			OutputBudget: req.OutputBudget,
		},
		Ollama: OllamaData{
			PromptTokens:         req.PromptTokens,
			CompletionTokens:     req.CompletionTokens,
			UpstreamInBytes:      req.UpstreamInBytes,
			UpstreamOutBytes:     req.UpstreamOutBytes,
			UpstreamTotalMs:      req.UpstreamTotalMs,
			UpstreamLoadMs:       req.UpstreamLoadMs,
			UpstreamPromptEvalMs: req.UpstreamPromptEvalMs,
			UpstreamEvalMs:       req.UpstreamEvalMs,
			HTTPStatus:           req.UpstreamHTTPStatus,
		},
		Response: ResponseData{
			DurationMs:     req.DurationMs,
			TTFBMs:         req.TTFBMs,
			ClientOutBytes: req.ClientOutBytes,
			RetryCount:     req.RetryCount,
			ErrorClass:     req.ErrorClass,
		},
	}

	s.writeJSON(w, resp)
}

// ModelListResponse contains per-model statistics.
type ModelListResponse struct {
	Models []storage.ModelStat `json:"models"`
}

// handleListModels returns per-model rollup statistics.
// GET /autoctx/api/v1/models?window=24h|7d
func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		s.writeError(w, http.StatusServiceUnavailable, "storage not available")
		return
	}

	window := parseWindow(r)
	stats, err := s.store.ModelStats(window)
	if err != nil {
		s.logger.Error("failed to get model stats", "err", err)
		s.writeError(w, http.StatusInternalServerError, "failed to get model stats")
		return
	}

	s.writeJSON(w, ModelListResponse{Models: stats})
}

// ModelSeriesResponse contains time series data for a model.
type ModelSeriesResponse struct {
	Model  string              `json:"model"`
	Series []storage.DataPoint `json:"series"`
	Metric string              `json:"metric"`
}

// handleModelSeries returns time-binned data for a specific model.
// GET /autoctx/api/v1/models/{model}/series?window=1h|24h|7d&metric=duration_p95|gen_tok_per_s|ctx_utilization|req_count
func (s *Server) handleModelSeries(w http.ResponseWriter, r *http.Request, model string) {
	if s.store == nil {
		s.writeError(w, http.StatusServiceUnavailable, "storage not available")
		return
	}

	window := parseWindow(r)
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "duration_p95"
	}

	series, err := s.store.Series(storage.SeriesOptions{
		Window: window,
		Metric: metric,
		Model:  model,
	})
	if err != nil {
		s.logger.Error("failed to get model series", "err", err, "model", model)
		s.writeError(w, http.StatusInternalServerError, "failed to get model series")
		return
	}

	s.writeJSON(w, ModelSeriesResponse{
		Model:  model,
		Series: series,
		Metric: metric,
	})
}

// ConfigResponse contains current configuration.
type ConfigResponse struct {
	Mode           string `json:"mode"`
	Storage        string `json:"storage"`
	StorageMaxRows int    `json:"storage_max_rows"`
	RetryMax       int    `json:"retry_max"`
	Features       struct {
		Dashboard bool `json:"dashboard"`
		API       bool `json:"api"`
		Events    bool `json:"events"`
		Metrics   bool `json:"metrics"`
		Storage   bool `json:"storage"`
		Retry     bool `json:"retry"`
		Protect   bool `json:"protect"`
	} `json:"features"`
}

// handleConfig returns the current configuration.
// GET /autoctx/api/v1/config
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	features := s.cfg.Features()

	resp := ConfigResponse{
		Mode:           string(s.cfg.Mode),
		Storage:        string(s.cfg.Storage),
		StorageMaxRows: s.cfg.StorageMaxRows,
		RetryMax:       s.cfg.RetryMax,
	}
	resp.Features.Dashboard = features.Dashboard
	resp.Features.API = features.API
	resp.Features.Events = features.Events
	resp.Features.Metrics = features.Metrics
	resp.Features.Storage = features.Storage
	resp.Features.Retry = features.Retry
	resp.Features.Protect = features.Protect

	s.writeJSON(w, resp)
}
