package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"ollama-auto-ctx/internal/calibration"
	"ollama-auto-ctx/internal/config"
	"ollama-auto-ctx/internal/estimate"
	"ollama-auto-ctx/internal/ollama"
	"ollama-auto-ctx/internal/supervisor"
	"ollama-auto-ctx/internal/util"
)

type ctxKey string

const (
	ctxSampleKey      ctxKey = "sample"
	ctxDecisionKey    ctxKey = "decision"
	ctxClampedKey     ctxKey = "clamped"
	ctxRequestIDKey   ctxKey = "request_id"
	ctxCancelFuncKey  ctxKey = "cancel_func"
)

// Decision captures how the proxy chose a context size.
// It is stored in request context for logging and for response headers.
type Decision struct {
	Model                string
	Endpoint             string
	EstimatedPromptTokens int
	OutputBudgetTokens   int
	OutputBudgetSource   string // "explicit_num_predict", "dynamic_default", or "fixed_default"
	NeededTokens         int
	NeededWithHeadroom   int
	ChosenCtx            int
	UserCtx              int
	UserCtxProvided      bool
	OverrideApplied      bool
	Clamped              bool
	MaxConfigCtx         int
	MaxModelCtx          int
	MaxSafeCtx           int
	ThinkVerdict         string
}

// Handler is an http.Handler that proxies to Ollama and injects options.num_ctx
// for /api/chat and /api/generate.
type Handler struct {
	cfg          config.Config
	logger       *slog.Logger
	proxy        *httputil.ReverseProxy
	showCache    *ollama.ShowCache
	calib        *calibration.Store
	tracker      *supervisor.Tracker
	watchdog     *supervisor.Watchdog
	eventBus     *supervisor.EventBus
	retryer      *supervisor.Retryer
	metrics      *supervisor.Metrics
	healthChecker *supervisor.HealthChecker
	upstream     *url.URL
	nextID       int64 // atomic counter for request IDs
}

func (h *Handler) modifyResponse(resp *http.Response) error {
	// If the request was clamped by model/config limits, expose that as a response header.
	if clamped, ok := resp.Request.Context().Value(ctxClampedKey).(bool); ok && clamped {
		resp.Header.Set("X-Ollama-CtxProxy-Clamped", "true")
	}

	// Calibration is optional and should never break the request.
	if !h.cfg.CalibrationEnabled {
		return nil
	}
	sample, ok := resp.Request.Context().Value(ctxSampleKey).(calibration.Sample)
	if !ok || sample.Model == "" {
		return nil
	}

	ct := resp.Header.Get("Content-Type")
	// Get request ID from context
	reqID := ""
	if reqIDVal := resp.Request.Context().Value(ctxRequestIDKey); reqIDVal != nil {
		if id, ok := reqIDVal.(string); ok {
			reqID = id
		}
	}

	// Create loop detector if enabled and we have a cancel function
	var loopDetector *supervisor.LoopDetector
	var cancelFunc func()
	if h.cfg.SupervisorLoopDetectEnabled {
		if cancelFuncVal := resp.Request.Context().Value(ctxCancelFuncKey); cancelFuncVal != nil {
			if cancel, ok := cancelFuncVal.(context.CancelFunc); ok {
				cancelFunc = cancel
				loopDetector = supervisor.NewLoopDetector(
					supervisor.LoopDetectorConfig{
						WindowBytes:     h.cfg.SupervisorLoopWindowBytes,
						NgramBytes:      h.cfg.SupervisorLoopNgramBytes,
						RepeatThreshold: h.cfg.SupervisorLoopRepeatThreshold,
						MinOutputBytes:  h.cfg.SupervisorLoopMinOutputBytes,
					},
					reqID,
					cancel,
					h.tracker,
				)
			}
		}
	}

	// Get cancel function for output limit if not already set
	if cancelFunc == nil {
		if cancelFuncVal := resp.Request.Context().Value(ctxCancelFuncKey); cancelFuncVal != nil {
			if cancel, ok := cancelFuncVal.(context.CancelFunc); ok {
				cancelFunc = cancel
			}
		}
	}

	// Determine output limit parameters
	var outputTokenLimit int64
	var outputLimitAction string
	var minOutputBytes int64
	if h.cfg.SupervisorOutputSafetyLimitEnabled && h.cfg.SupervisorOutputSafetyLimitTokens > 0 {
		outputTokenLimit = int64(h.cfg.SupervisorOutputSafetyLimitTokens)
		outputLimitAction = h.cfg.SupervisorOutputSafetyLimitAction
		// Use same minimum as loop detection for consistency
		minOutputBytes = int64(h.cfg.SupervisorLoopMinOutputBytes)
		if minOutputBytes < 256 {
			minOutputBytes = 256 // Minimum enforced by loop detector
		}
	}

	resp.Body = NewTapReadCloser(resp.Body, ct, resp.ContentLength, h.cfg.ResponseTapMaxBytes, sample, h.calib, h.tracker, loopDetector, reqID, h.logger, outputTokenLimit, outputLimitAction, cancelFunc, minOutputBytes)
	return nil
}

// NewHandler constructs the proxy handler.
func NewHandler(cfg config.Config, upstream *url.URL, showCache *ollama.ShowCache, calib *calibration.Store, tracker *supervisor.Tracker, watchdog *supervisor.Watchdog, eventBus *supervisor.EventBus, retryer *supervisor.Retryer, metrics *supervisor.Metrics, healthChecker *supervisor.HealthChecker, logger *slog.Logger) *Handler {
	rp := httputil.NewSingleHostReverseProxy(upstream)
	rp.FlushInterval = cfg.FlushInterval

	h := &Handler{
		cfg:          cfg,
		upstream:     upstream,
		logger:       logger,
		proxy:        rp,
		showCache:    showCache,
		calib:        calib,
		tracker:      tracker,
		watchdog:     watchdog,
		eventBus:     eventBus,
		retryer:      retryer,
		metrics:      metrics,
		healthChecker: healthChecker,
	}

	// Tap responses for calibration and add headers without buffering streams.
	rp.ModifyResponse = h.modifyResponse

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("upstream proxy error", "err", err, "path", r.URL.Path)

		// Mark request as failed and stop watchdog if tracking is enabled
		if reqIDVal := r.Context().Value(ctxRequestIDKey); reqIDVal != nil {
			if reqID, ok := reqIDVal.(string); ok {
				if h.tracker != nil {
					h.tracker.Finish(reqID, supervisor.StatusUpstreamError, err)
				}
				if h.watchdog != nil {
					h.watchdog.Stop(reqID)
				}
			}
		}

		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	}

	return h
}

// ServeHTTP implements the proxy + rewrite logic.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Metrics endpoint
	if h.cfg.SupervisorMetricsEnabled && r.URL.Path == h.cfg.SupervisorMetricsPath && r.Method == http.MethodGet {
		h.handleMetrics(w, r)
		return
	}

	// Health endpoints
	if r.URL.Path == "/healthz" {
		h.handleHealthz(w, r)
		return
	}
	if r.URL.Path == "/healthz/upstream" {
		h.handleHealthzUpstream(w, r)
		return
	}

	// Dashboard endpoint
	if r.URL.Path == "/dashboard" && r.Method == http.MethodGet {
		h.handleDashboard(w, r)
		return
	}

	// Generate request ID for tracking
	reqID := h.generateRequestID()

	// Build context chain: start with request context, add request ID, optionally add cancellation
	ctx := r.Context()
	ctx = context.WithValue(ctx, ctxRequestIDKey, reqID)

	// Track if request was already finished (by watchdog timeout or error handler)
	var alreadyFinished bool

	// Start tracking if enabled
	if h.tracker != nil {
		endpoint := ""
		if r.Method == http.MethodPost && r.URL.Path == "/api/chat" {
			endpoint = estimate.EndpointChat
		} else if r.Method == http.MethodPost && r.URL.Path == "/api/generate" {
			endpoint = estimate.EndpointGenerate
		}

		stream := r.URL.Query().Get("stream") == "true"
		h.tracker.Start(reqID, endpoint, "", stream) // model will be determined later

		// Ensure request is marked as finished on all exit paths (unless already finished)
		defer func() {
			if !alreadyFinished && h.tracker.GetRequestInfo(reqID) != nil {
				h.tracker.Finish(reqID, supervisor.StatusSuccess, nil)
			}
		}()
	}

	// Set up context cancellation for watchdog and/or loop detection
	var cancel context.CancelFunc
	needsCancel := h.watchdog != nil || h.cfg.SupervisorLoopDetectEnabled
	if needsCancel {
		ctx, cancel = context.WithCancel(ctx)
		// Store cancel func in context for loop detector to use
		ctx = context.WithValue(ctx, ctxCancelFuncKey, cancel)

		if h.watchdog != nil {
			h.watchdog.Start(reqID, cancel)
			// Ensure watchdog stops monitoring when request finishes
			defer h.watchdog.Stop(reqID)
		}
	}

	// Apply the complete context to the request
	*r = *r.WithContext(ctx)
	_ = cancel // silence unused warning when cancel is nil

	// Observability endpoints (before proxy delegation)
	if h.cfg.SupervisorObsEnabled {
		if r.URL.Path == "/debug/requests" && h.cfg.SupervisorObsRequestsEndpoint && r.Method == http.MethodGet {
			h.handleDebugRequests(w, r)
			return
		}
		if r.URL.Path == "/events" && h.cfg.SupervisorObsSSEEndpoint && r.Method == http.MethodGet {
			h.handleSSEEvents(w, r)
			return
		}
	}

	// Simple CORS support (can be disabled by setting CORS_ALLOW_ORIGIN="").
	if h.cfg.CORSAllowOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", h.cfg.CORSAllowOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Expose-Headers", "X-Ollama-CtxProxy-Clamped")
	}
	if r.Method == http.MethodOptions {
		// Preflight: reply quickly.
		if r.Header.Get("Access-Control-Request-Method") != "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	// Only rewrite the two core endpoints. Everything else is pass-through.
	endpoint := ""
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/chat":
		endpoint = estimate.EndpointChat
	case r.Method == http.MethodPost && r.URL.Path == "/api/generate":
		endpoint = estimate.EndpointGenerate
	}

	if endpoint != "" {
		h.rewriteRequestIfPossible(endpoint, r)
	}

	// Delegate to reverse proxy.
	h.proxy.ServeHTTP(w, r)
}

func (h *Handler) rewriteRequestIfPossible(endpoint string, r *http.Request) {
	// Only attempt if the request body is small enough and likely JSON.
	if r.Body == nil {
		return
	}
	if r.ContentLength < 0 || r.ContentLength > h.cfg.RequestBodyMaxBytes {
		// Unknown/large bodies: don't parse; pass-through.
		return
	}
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "application/json") {
		// Some clients omit content-type; if set and not JSON, don't touch.
		return
	}

	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		return
	}

	// Always restore the body, even if parsing fails.
	setBody(r, body)

	reqMap, err := util.DecodeJSONMap(body)
	if err != nil {
		// Invalid JSON: pass-through unchanged.
		return
	}

	// Extract thinking directive from system prompt (this also strips it from the prompt)
	systemPromptThinkVerdict := estimate.ExtractThinkingFromSystemPrompt(reqMap, endpoint)

	// Strip system prompt text if configured
	if h.cfg.StripSystemPromptText != "" {
		estimate.StripSystemPromptText(reqMap, endpoint, h.cfg.StripSystemPromptText)
	}

	features, err := estimate.ExtractFeatures(endpoint, reqMap)
	if err != nil {
		return
	}
	if features.Model == "" {
		return
	}

	// Update tracker with model information if tracking is enabled
	if h.tracker != nil {
		if reqIDVal := r.Context().Value(ctxRequestIDKey); reqIDVal != nil {
			if reqID, ok := reqIDVal.(string); ok {
				h.tracker.UpdateModel(reqID, features.Model)
			}
		}
	}

	// Fetch model limits (cached). If this fails, we fall back to config max.
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	show, showErr := h.showCache.Get(ctx, features.Model)
	maxModelCtx, _ := show.MaxContextLength()
	if showErr != nil {
		// Don't fail request, just log at debug.
		h.logger.Debug("/api/show failed; using config max only", "model", features.Model, "err", showErr)
	}

	// Tokens-per-image: use model value if available, else fallback.
	tokensPerImage, ok := show.TokensPerImage()
	if !ok {
		tokensPerImage = h.cfg.DefaultTokensPerImageFallback
	}

	params := h.calib.Get(features.Model)

	// Effective max is the min of (config max, model max, per-model safe max (if set)).
	effMax := h.cfg.MaxCtx
	maxSafe := 0
	if params.SafeMaxCtx > 0 {
		maxSafe = params.SafeMaxCtx
		if maxSafe < effMax {
			effMax = maxSafe
		}
	}
	if maxModelCtx > 0 && maxModelCtx < effMax {
		effMax = maxModelCtx
	}
	// Effective min cannot exceed effective max.
	effMin := h.cfg.MinCtx
	if effMax > 0 && effMin > effMax {
		effMin = effMax
	}

	promptTokens := estimate.EstimatePromptTokens(features, params, tokensPerImage)
	budgetResult := estimate.BudgetOutputTokens(features, h.cfg.DefaultOutputBudget, h.cfg.MaxOutputBudget, h.cfg.StructuredOverhead, h.cfg.DynamicDefaultOutputBudget, promptTokens)
	outputBudget := budgetResult.Budget
	needed := promptTokens + outputBudget
	neededHeadroom := estimate.ApplyHeadroom(needed, h.cfg.Headroom)
	bucket := estimate.Bucketize(neededHeadroom, h.cfg.Buckets)
	desiredCtx := estimate.ClampCtx(bucket, effMin, effMax)

	// Decide final context window based on override policy and any user-provided value.
	finalCtx, override, clamped := chooseFinalCtx(desiredCtx, effMax, features.ProvidedNumCtx, features.ProvidedNumCtxOK, h.cfg.OverrideNumCtx)

	// Validate system prompt thinking verdict against model family
	finalThinkVerdict := ""
	if systemPromptThinkVerdict != "" {
		modelLower := strings.ToLower(features.Model)
		if strings.HasPrefix(modelLower, "qwen3") || strings.HasPrefix(modelLower, "deepseek") {
			// Boolean type: only accept "true" or "false"
			if systemPromptThinkVerdict == "true" || systemPromptThinkVerdict == "false" {
				finalThinkVerdict = systemPromptThinkVerdict
			}
		} else if strings.HasPrefix(modelLower, "gpt-oss") {
			// Level type: only accept "low", "medium", or "high"
			if systemPromptThinkVerdict == "low" || systemPromptThinkVerdict == "medium" || systemPromptThinkVerdict == "high" {
				finalThinkVerdict = systemPromptThinkVerdict
			}
		}
	}

	// Prepare to modify request body if needed
	needsRewrite := override || clamped || finalThinkVerdict != ""

	// If we need to inject/adjust options.num_ctx or thinking mode, do it.
	if needsRewrite {
		// Inject num_ctx if needed
		if override || clamped {
			opt, ok := reqMap["options"].(map[string]any)
			if !ok || opt == nil {
				opt = make(map[string]any)
			}
			opt["num_ctx"] = finalCtx
			reqMap["options"] = opt
		}

		// Inject thinking option at top level if we have a verdict
		if finalThinkVerdict != "" {
			modelLower := strings.ToLower(features.Model)
			if strings.HasPrefix(modelLower, "qwen3") || strings.HasPrefix(modelLower, "deepseek") {
				// Boolean type: convert string to bool
				reqMap["think"] = (finalThinkVerdict == "true")
			} else if strings.HasPrefix(modelLower, "gpt-oss") {
				// Level type: pass string directly
				reqMap["think"] = finalThinkVerdict
			}
		}

		newBody, err := util.EncodeJSON(reqMap)
		if err != nil {
			return
		}
		setBody(r, newBody)
	}

	// Store sample + decision in request context for response tapping/logging.
	imageTokens := tokensPerImage * features.ImageCount
	sample := calibration.Sample{
		Model:        features.Model,
		Endpoint:     endpoint,
		TextBytes:    features.TextBytes,
		MessageCount: features.MessageCount,
		ImageTokens:  imageTokens,
		UsedCtx:      finalCtx,
		CreatedAt:    time.Now(),
	}
	dec := Decision{
		Model:                 features.Model,
		Endpoint:              endpoint,
		EstimatedPromptTokens: promptTokens,
		OutputBudgetTokens:    outputBudget,
		OutputBudgetSource:    budgetResult.Source,
		NeededTokens:          needed,
		NeededWithHeadroom:    neededHeadroom,
		ChosenCtx:             finalCtx,
		UserCtx:               features.ProvidedNumCtx,
		UserCtxProvided:       features.ProvidedNumCtxOK,
		OverrideApplied:       override,
		Clamped:               clamped,
		MaxConfigCtx:          h.cfg.MaxCtx,
		MaxModelCtx:           maxModelCtx,
		MaxSafeCtx:            maxSafe,
		ThinkVerdict:          finalThinkVerdict,
	}

	ctx2 := context.WithValue(r.Context(), ctxSampleKey, sample)
	ctx2 = context.WithValue(ctx2, ctxDecisionKey, dec)
	if clamped {
		ctx2 = context.WithValue(ctx2, ctxClampedKey, true)
	}
	*r = *r.WithContext(ctx2)

	// Structured log for the decision.
	h.logger.Info("ctx decision",
		"path", r.URL.Path,
		"model", dec.Model,
		"endpoint", dec.Endpoint,
		"prompt_tokens_est", dec.EstimatedPromptTokens,
		"output_budget", dec.OutputBudgetTokens,
		"output_budget_source", dec.OutputBudgetSource,
		"needed", dec.NeededTokens,
		"needed_headroom", dec.NeededWithHeadroom,
		"chosen_ctx", dec.ChosenCtx,
		"user_ctx", dec.UserCtx,
		"user_ctx_provided", dec.UserCtxProvided,
		"override_applied", dec.OverrideApplied,
		"clamped", dec.Clamped,
		"max_model_ctx", dec.MaxModelCtx,
		"max_safe_ctx", dec.MaxSafeCtx,
		"think_verdict", dec.ThinkVerdict,
	)
}

// chooseFinalCtx applies the override policy and hard clamps (model/config safety).
//
// Returns:
// - finalCtx:  the context window that will actually be used upstream
// - override:  whether we should inject/overwrite options.num_ctx in the request
// - clamped:   whether we had to forcibly clamp a user-provided ctx down to a hard maximum
func chooseFinalCtx(desiredCtx, hardMax int, userCtx int, userProvided bool, policy config.OverridePolicy) (finalCtx int, override bool, clamped bool) {
	// desiredCtx is already clamped to [effMin, hardMax].
	finalCtx = desiredCtx

	// If user provides a ctx larger than hardMax, we MUST clamp or the request may fail/oom.
	if userProvided && hardMax > 0 && userCtx > hardMax {
		finalCtx = hardMax
		override = true
		clamped = true
		return
	}

	if !userProvided {
		// No user ctx: we always set one to make behavior deterministic.
		return desiredCtx, true, false
	}

	switch policy {
	case config.OverrideAlways:
		return desiredCtx, true, false
	case config.OverrideIfMissing:
		return userCtx, false, false
	case config.OverrideIfTooSmall:
		if userCtx < desiredCtx {
			return desiredCtx, true, false
		}
		return userCtx, false, false
	default:
		// Be conservative.
		if userCtx < desiredCtx {
			return desiredCtx, true, false
		}
		return userCtx, false, false
	}
}

func setBody(r *http.Request, b []byte) {
	r.Body = io.NopCloser(bytes.NewReader(b))
	r.ContentLength = int64(len(b))
	r.Header.Set("Content-Length", intToString(len(b)))
}

func intToString(n int) string {
	return strconv.Itoa(n)
}

// generateRequestID generates a unique request ID using an atomic counter.
func (h *Handler) generateRequestID() string {
	id := atomic.AddInt64(&h.nextID, 1)
	return strconv.FormatInt(id, 10)
}

// handleDebugRequests returns the current state of in-flight and recent requests.
func (h *Handler) handleDebugRequests(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		http.Error(w, "tracker not available", http.StatusServiceUnavailable)
		return
	}

	snapshot := h.tracker.Snapshot()

	// Enrich with estimated output tokens
	type EnrichedRequestInfo struct {
		supervisor.RequestInfo
		EstimatedOutputTokens int64 `json:"estimated_output_tokens"`
	}

	type Response struct {
		InFlight map[string]EnrichedRequestInfo `json:"in_flight"`
		Recent   []EnrichedRequestInfo          `json:"recent"`
	}

	resp := Response{
		InFlight: make(map[string]EnrichedRequestInfo, len(snapshot.InFlight)),
		Recent:   make([]EnrichedRequestInfo, 0, len(snapshot.Recent)),
	}

	// Enrich in-flight requests
	for id, req := range snapshot.InFlight {
		estTokens := supervisor.EstimateOutputTokens(req.BytesForwarded, req.Model, h.calib, h.cfg.DefaultTokensPerByte)
		resp.InFlight[id] = EnrichedRequestInfo{
			RequestInfo:           req,
			EstimatedOutputTokens: estTokens,
		}
	}

	// Enrich recent requests
	for _, req := range snapshot.Recent {
		estTokens := supervisor.EstimateOutputTokens(req.BytesForwarded, req.Model, h.calib, h.cfg.DefaultTokensPerByte)
		resp.Recent = append(resp.Recent, EnrichedRequestInfo{
			RequestInfo:           req,
			EstimatedOutputTokens: estTokens,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode debug requests response", "err", err)
	}
}

// handleSSEEvents streams Server-Sent Events for real-time request monitoring.
func (h *Handler) handleSSEEvents(w http.ResponseWriter, r *http.Request) {
	if h.eventBus == nil {
		http.Error(w, "event bus not available", http.StatusServiceUnavailable)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	if h.cfg.CORSAllowOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", h.cfg.CORSAllowOrigin)
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Subscribe to event bus
	eventCh := h.eventBus.Subscribe()
	defer h.eventBus.Unsubscribe(eventCh)

	// Send initial state marker (SSE comment)
	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	// Stream events
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return
		case event, ok := <-eventCh:
			if !ok {
				// Channel closed
				return
			}
			sseData, err := supervisor.FormatSSEEvent(event)
			if err != nil {
				h.logger.Error("failed to format SSE event", "err", err)
				continue
			}
			if _, err := w.Write([]byte(sseData)); err != nil {
				// Client disconnected
				return
			}
			flusher.Flush()
		}
	}
}

// handleMetrics serves Prometheus metrics.
func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if h.metrics == nil {
		http.Error(w, "metrics not enabled", http.StatusServiceUnavailable)
		return
	}

	// Use Prometheus handler to serve metrics
	promhttp.Handler().ServeHTTP(w, r)
}

// handleHealthz serves the combined proxy + upstream health check.
func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	// Check upstream health if health checker is enabled
	if h.healthChecker != nil && !h.healthChecker.Healthy() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("upstream unhealthy"))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleHealthzUpstream serves upstream-only health status with details.
func (h *Handler) handleHealthzUpstream(w http.ResponseWriter, r *http.Request) {
	if h.healthChecker == nil {
		http.Error(w, "health check not enabled", http.StatusServiceUnavailable)
		return
	}

	healthy := h.healthChecker.Healthy()
	lastCheck := h.healthChecker.LastCheck()
	lastError := h.healthChecker.LastError()

	w.Header().Set("Content-Type", "application/json")
	if healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	response := map[string]interface{}{
		"healthy":   healthy,
		"last_check": lastCheck.Format(time.RFC3339),
	}
	if lastError != "" {
		response["last_error"] = lastError
	}

	json.NewEncoder(w).Encode(response)
}

// handleDashboard serves the monitoring dashboard HTML page.
func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(dashboardHTML))
}
