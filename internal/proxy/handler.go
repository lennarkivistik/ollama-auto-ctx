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

	"ollama-auto-ctx/internal/api"
	"ollama-auto-ctx/internal/calibration"
	"ollama-auto-ctx/internal/config"
	"ollama-auto-ctx/internal/estimate"
	"ollama-auto-ctx/internal/ollama"
	"ollama-auto-ctx/internal/storage"
	"ollama-auto-ctx/internal/supervisor"
	"ollama-auto-ctx/internal/util"
)

type ctxKey string

const (
	ctxSampleKey     ctxKey = "sample"
	ctxDecisionKey   ctxKey = "decision"
	ctxClampedKey    ctxKey = "clamped"
	ctxRequestIDKey  ctxKey = "request_id"
	ctxCancelFuncKey ctxKey = "cancel_func"
	ctxMetadataKey   ctxKey = "metadata"
)

// Decision captures how the proxy chose a context size.
type Decision struct {
	Model                 string
	Endpoint              string
	EstimatedPromptTokens int
	OutputBudgetTokens    int
	OutputBudgetSource    string
	NeededTokens          int
	NeededWithHeadroom    int
	ChosenCtx             int
	UserCtx               int
	UserCtxProvided       bool
	OverrideApplied       bool
	Clamped               bool
	MaxConfigCtx          int
	MaxModelCtx           int
	MaxSafeCtx            int
	ThinkVerdict          string
}

// Handler is an http.Handler that proxies to Ollama and injects options.num_ctx.
type Handler struct {
	cfg           config.Config
	features      config.Features
	logger        *slog.Logger
	proxy         *httputil.ReverseProxy
	showCache     *ollama.ShowCache
	calib         *calibration.Store
	store         storage.Store
	apiServer     *api.Server
	tracker       *supervisor.Tracker
	watchdog      *supervisor.Watchdog
	eventBus      *supervisor.EventBus
	retryer       *supervisor.Retryer
	metrics       *supervisor.Metrics
	healthChecker *supervisor.HealthChecker
	upstream      *url.URL
	nextID        int64
}

// NewHandler constructs the proxy handler.
func NewHandler(
	cfg config.Config,
	features config.Features,
	upstream *url.URL,
	showCache *ollama.ShowCache,
	calib *calibration.Store,
	store storage.Store,
	apiServer *api.Server,
	tracker *supervisor.Tracker,
	watchdog *supervisor.Watchdog,
	eventBus *supervisor.EventBus,
	retryer *supervisor.Retryer,
	metrics *supervisor.Metrics,
	healthChecker *supervisor.HealthChecker,
	logger *slog.Logger,
) *Handler {
	rp := httputil.NewSingleHostReverseProxy(upstream)
	rp.FlushInterval = cfg.FlushInterval

	h := &Handler{
		cfg:           cfg,
		features:      features,
		upstream:      upstream,
		logger:        logger,
		proxy:         rp,
		showCache:     showCache,
		calib:         calib,
		store:         store,
		apiServer:     apiServer,
		tracker:       tracker,
		watchdog:      watchdog,
		eventBus:      eventBus,
		retryer:       retryer,
		metrics:       metrics,
		healthChecker: healthChecker,
	}

	rp.ModifyResponse = h.modifyResponse

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("upstream proxy error", "err", err, "path", r.URL.Path)

		if reqIDVal := r.Context().Value(ctxRequestIDKey); reqIDVal != nil {
			if reqID, ok := reqIDVal.(string); ok {
				if h.tracker != nil {
					h.tracker.Finish(reqID, supervisor.StatusUpstreamError, err)
				}
				if h.watchdog != nil {
					h.watchdog.Stop(reqID)
				}
				// Update storage
				if h.store != nil {
					now := time.Now().UnixMilli()
					status := storage.StatusError
					reason := storage.ReasonUpstreamError
					h.store.Update(reqID, storage.RequestUpdate{
						TSEnd:  &now,
						Status: &status,
						Reason: &reason,
					})
				}
			}
		}

		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	}

	return h
}

func (h *Handler) modifyResponse(resp *http.Response) error {
	if clamped, ok := resp.Request.Context().Value(ctxClampedKey).(bool); ok && clamped {
		resp.Header.Set("X-Ollama-CtxProxy-Clamped", "true")
	}

	if !h.cfg.CalibrationEnabled {
		return nil
	}
	sample, ok := resp.Request.Context().Value(ctxSampleKey).(calibration.Sample)
	if !ok || sample.Model == "" {
		return nil
	}

	ct := resp.Header.Get("Content-Type")
	reqID := ""
	if reqIDVal := resp.Request.Context().Value(ctxRequestIDKey); reqIDVal != nil {
		if id, ok := reqIDVal.(string); ok {
			reqID = id
		}
	}

	// Loop detector for protect mode
	var loopDetector *supervisor.LoopDetector
	var cancelFunc func()
	if h.features.Protect && h.cfg.LoopDetectEnabled {
		if cancelFuncVal := resp.Request.Context().Value(ctxCancelFuncKey); cancelFuncVal != nil {
			if cancel, ok := cancelFuncVal.(context.CancelFunc); ok {
				cancelFunc = cancel
				loopDetector = supervisor.NewLoopDetector(
					supervisor.LoopDetectorConfig{
						WindowBytes:     h.cfg.LoopWindowBytes,
						NgramBytes:      h.cfg.LoopNgramBytes,
						RepeatThreshold: h.cfg.LoopRepeatThreshold,
						MinOutputBytes:  h.cfg.LoopMinOutputBytes,
					},
					reqID,
					cancel,
					h.tracker,
				)
			}
		}
	}

	if cancelFunc == nil {
		if cancelFuncVal := resp.Request.Context().Value(ctxCancelFuncKey); cancelFuncVal != nil {
			if cancel, ok := cancelFuncVal.(context.CancelFunc); ok {
				cancelFunc = cancel
			}
		}
	}

	// Output limit for protect mode
	var outputTokenLimit int64
	var outputLimitAction string
	var minOutputBytes int64
	if h.features.Protect && h.cfg.OutputLimitEnabled && h.cfg.OutputLimitMaxTokens > 0 {
		outputTokenLimit = int64(h.cfg.OutputLimitMaxTokens)
		outputLimitAction = "cancel"
		minOutputBytes = int64(h.cfg.LoopMinOutputBytes)
		if minOutputBytes < 256 {
			minOutputBytes = 256
		}
	}

	resp.Body = NewTapReadCloser(resp.Body, ct, resp.ContentLength, h.cfg.ResponseTapMaxBytes,
		sample, h.calib, h.tracker, loopDetector, reqID, h.logger,
		outputTokenLimit, outputLimitAction, cancelFunc, minOutputBytes)
	return nil
}

// ServeHTTP implements the proxy + rewrite logic.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// API endpoints (when enabled)
	if h.features.API && h.apiServer != nil && h.apiServer.Handles(r.URL.Path) {
		h.apiServer.ServeHTTP(w, r)
		return
	}

	// Metrics endpoint
	if h.features.Metrics && r.URL.Path == "/metrics" && r.Method == http.MethodGet {
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

	// Dashboard (when enabled)
	if h.features.Dashboard && r.URL.Path == "/dashboard" && r.Method == http.MethodGet {
		h.handleDashboard(w, r)
		return
	}

	// Events SSE (when enabled)
	if h.features.Events && r.URL.Path == "/events" && r.Method == http.MethodGet {
		h.handleSSEEvents(w, r)
		return
	}

	// Legacy debug endpoint (redirect to new API)
	if r.URL.Path == "/debug/requests" && r.Method == http.MethodGet {
		if h.features.API && h.apiServer != nil {
			http.Redirect(w, r, "/autoctx/api/v1/requests", http.StatusTemporaryRedirect)
			return
		}
		h.handleDebugRequests(w, r)
		return
	}

	// Only track Ollama API endpoints
	isOllamaEndpoint := (r.Method == http.MethodPost && r.URL.Path == "/api/chat") ||
		(r.Method == http.MethodPost && r.URL.Path == "/api/generate")

	var reqID string
	if isOllamaEndpoint {
		reqID = h.generateRequestID()
	}

	ctx := r.Context()
	if isOllamaEndpoint {
		ctx = context.WithValue(ctx, ctxRequestIDKey, reqID)
	}

	var alreadyFinished bool

	// Start tracking
	if h.tracker != nil && isOllamaEndpoint {
		endpoint := ""
		if r.URL.Path == "/api/chat" {
			endpoint = estimate.EndpointChat
		} else if r.URL.Path == "/api/generate" {
			endpoint = estimate.EndpointGenerate
		}

		stream := r.URL.Query().Get("stream") == "true"
		h.tracker.Start(reqID, endpoint, "", stream)

		defer func() {
			if !alreadyFinished && h.tracker.GetRequestInfo(reqID) != nil {
				h.tracker.Finish(reqID, supervisor.StatusSuccess, nil)
			}
		}()
	}

	// Context cancellation for watchdog/loop detection
	var cancel context.CancelFunc
	needsCancel := isOllamaEndpoint && (h.watchdog != nil || (h.features.Protect && h.cfg.LoopDetectEnabled))
	if needsCancel {
		ctx, cancel = context.WithCancel(ctx)
		ctx = context.WithValue(ctx, ctxCancelFuncKey, cancel)

		if h.watchdog != nil {
			h.watchdog.Start(reqID, cancel)
			defer h.watchdog.Stop(reqID)
		}
	}

	*r = *r.WithContext(ctx)
	_ = cancel

	// CORS
	if h.cfg.CORSAllowOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", h.cfg.CORSAllowOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Expose-Headers", "X-Ollama-CtxProxy-Clamped")
	}
	if r.Method == http.MethodOptions {
		if r.Header.Get("Access-Control-Request-Method") != "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	// Only rewrite the two core endpoints
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

	h.proxy.ServeHTTP(w, r)
}

func (h *Handler) rewriteRequestIfPossible(endpoint string, r *http.Request) {
	if r.Body == nil {
		return
	}
	if r.ContentLength < 0 || r.ContentLength > h.cfg.RequestBodyMaxBytes {
		return
	}
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "application/json") {
		return
	}

	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		return
	}

	setBody(r, body)

	reqMap, err := util.DecodeJSONMap(body)
	if err != nil {
		return
	}

	// Parse metadata for storage
	if h.store != nil {
		meta := ParseRequestMetadata(endpoint, reqMap, len(body))
		reqID := ""
		if reqIDVal := r.Context().Value(ctxRequestIDKey); reqIDVal != nil {
			reqID, _ = reqIDVal.(string)
		}
		if reqID != "" {
			storageReq := meta.ToStorageRequest(reqID, time.Now().UnixMilli())
			if err := h.store.Insert(storageReq); err != nil {
				h.logger.Error("failed to insert request to storage", "err", err)
			}
		}
	}

	systemPromptThinkVerdict := estimate.ExtractThinkingFromSystemPrompt(reqMap, endpoint)

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

	// Update tracker with model
	if h.tracker != nil {
		if reqIDVal := r.Context().Value(ctxRequestIDKey); reqIDVal != nil {
			if reqID, ok := reqIDVal.(string); ok {
				h.tracker.UpdateModel(reqID, features.Model)
			}
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	show, showErr := h.showCache.Get(ctx, features.Model)
	maxModelCtx, _ := show.MaxContextLength()
	if showErr != nil {
		h.logger.Debug("/api/show failed; using config max only", "model", features.Model, "err", showErr)
	}

	tokensPerImage, ok := show.TokensPerImage()
	if !ok {
		tokensPerImage = h.cfg.DefaultTokensPerImageFallback
	}

	params := h.calib.Get(features.Model)

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

	finalCtx, override, clamped := chooseFinalCtx(desiredCtx, effMax, features.ProvidedNumCtx, features.ProvidedNumCtxOK, h.cfg.OverrideNumCtx)

	finalThinkVerdict := ""
	if systemPromptThinkVerdict != "" {
		modelLower := strings.ToLower(features.Model)
		if strings.HasPrefix(modelLower, "qwen3") || strings.HasPrefix(modelLower, "deepseek") {
			if systemPromptThinkVerdict == "true" || systemPromptThinkVerdict == "false" {
				finalThinkVerdict = systemPromptThinkVerdict
			}
		} else if strings.HasPrefix(modelLower, "gpt-oss") {
			if systemPromptThinkVerdict == "low" || systemPromptThinkVerdict == "medium" || systemPromptThinkVerdict == "high" {
				finalThinkVerdict = systemPromptThinkVerdict
			}
		}
	}

	needsRewrite := override || clamped || finalThinkVerdict != ""

	if needsRewrite {
		if override || clamped {
			opt, ok := reqMap["options"].(map[string]any)
			if !ok || opt == nil {
				opt = make(map[string]any)
			}
			opt["num_ctx"] = finalCtx
			reqMap["options"] = opt
		}

		if finalThinkVerdict != "" {
			modelLower := strings.ToLower(features.Model)
			if strings.HasPrefix(modelLower, "qwen3") || strings.HasPrefix(modelLower, "deepseek") {
				reqMap["think"] = (finalThinkVerdict == "true")
			} else if strings.HasPrefix(modelLower, "gpt-oss") {
				reqMap["think"] = finalThinkVerdict
			}
		}

		newBody, err := util.EncodeJSON(reqMap)
		if err != nil {
			return
		}
		setBody(r, newBody)
	}

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

	// Update tracker and storage with context data
	if reqIDVal := r.Context().Value(ctxRequestIDKey); reqIDVal != nil {
		if reqID, ok := reqIDVal.(string); ok {
			if h.tracker != nil {
				h.tracker.UpdateContextData(reqID, dec.EstimatedPromptTokens, dec.ChosenCtx, dec.OutputBudgetTokens)
			}
			if h.store != nil {
				ctxEst := dec.EstimatedPromptTokens
				ctxSelected := dec.ChosenCtx
				ctxBucket := bucket
				outBudget := dec.OutputBudgetTokens
				h.store.Update(reqID, storage.RequestUpdate{
					CtxEst:       &ctxEst,
					CtxSelected:  &ctxSelected,
					CtxBucket:    &ctxBucket,
					OutputBudget: &outBudget,
				})
			}
		}
	}

	h.logger.Info("ctx decision",
		"path", r.URL.Path,
		"model", dec.Model,
		"prompt_tokens_est", dec.EstimatedPromptTokens,
		"output_budget", dec.OutputBudgetTokens,
		"chosen_ctx", dec.ChosenCtx,
		"clamped", dec.Clamped,
	)
}

func chooseFinalCtx(desiredCtx, hardMax int, userCtx int, userProvided bool, policy config.OverridePolicy) (finalCtx int, override bool, clamped bool) {
	finalCtx = desiredCtx

	if userProvided && hardMax > 0 && userCtx > hardMax {
		finalCtx = hardMax
		override = true
		clamped = true
		return
	}

	if !userProvided {
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
		if userCtx < desiredCtx {
			return desiredCtx, true, false
		}
		return userCtx, false, false
	}
}

func setBody(r *http.Request, b []byte) {
	r.Body = io.NopCloser(bytes.NewReader(b))
	r.ContentLength = int64(len(b))
	r.Header.Set("Content-Length", strconv.Itoa(len(b)))
}

func (h *Handler) generateRequestID() string {
	id := atomic.AddInt64(&h.nextID, 1)
	return strconv.FormatInt(id, 10)
}

func (h *Handler) handleDebugRequests(w http.ResponseWriter, r *http.Request) {
	if h.tracker == nil {
		http.Error(w, "tracker not available", http.StatusServiceUnavailable)
		return
	}

	snapshot := h.tracker.Snapshot()

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

	for id, req := range snapshot.InFlight {
		estTokens := supervisor.EstimateOutputTokens(req.BytesForwarded, req.Model, h.calib, h.cfg.DefaultTokensPerByte)
		resp.InFlight[id] = EnrichedRequestInfo{
			RequestInfo:           req,
			EstimatedOutputTokens: estTokens,
		}
	}

	for _, req := range snapshot.Recent {
		estTokens := supervisor.EstimateOutputTokens(req.BytesForwarded, req.Model, h.calib, h.cfg.DefaultTokensPerByte)
		resp.Recent = append(resp.Recent, EnrichedRequestInfo{
			RequestInfo:           req,
			EstimatedOutputTokens: estTokens,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleSSEEvents(w http.ResponseWriter, r *http.Request) {
	if h.eventBus == nil {
		http.Error(w, "event bus not available", http.StatusServiceUnavailable)
		return
	}

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

	eventCh := h.eventBus.Subscribe()
	defer h.eventBus.Unsubscribe(eventCh)

	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			sseData, err := supervisor.FormatSSEEvent(event)
			if err != nil {
				continue
			}
			if _, err := w.Write([]byte(sseData)); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if h.metrics == nil {
		http.Error(w, "metrics not enabled", http.StatusServiceUnavailable)
		return
	}
	promhttp.Handler().ServeHTTP(w, r)
}

func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if h.healthChecker != nil && !h.healthChecker.Healthy() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("upstream unhealthy"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) handleHealthzUpstream(w http.ResponseWriter, r *http.Request) {
	if h.healthChecker == nil {
		// No health checker but still return something useful
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"healthy":    true,
			"last_check": time.Now().Format(time.RFC3339),
		})
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

	response := map[string]any{
		"healthy":    healthy,
		"last_check": lastCheck.Format(time.RFC3339),
	}
	if lastError != "" {
		response["last_error"] = lastError
	}

	json.NewEncoder(w).Encode(response)
}

func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(dashboardHTML))
}
