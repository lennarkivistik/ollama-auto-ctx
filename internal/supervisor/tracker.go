package supervisor

import (
	"sync"
	"time"

	"ollama-auto-ctx/internal/calibration"
)

// RequestStatus represents the final status of a request.
type RequestStatus string

const (
	StatusSuccess              RequestStatus = "success"
	StatusCanceled             RequestStatus = "canceled"
	StatusTimeout              RequestStatus = "timeout" // generic timeout
	StatusTimeoutTTFB          RequestStatus = "timeout_ttfb"
	StatusTimeoutStall         RequestStatus = "timeout_stall"
	StatusTimeoutHard          RequestStatus = "timeout_hard"
	StatusUpstreamError        RequestStatus = "upstream_error"
	StatusLoopDetected         RequestStatus = "loop_detected"
	StatusOutputLimitExceeded  RequestStatus = "output_limit_exceeded"
)

// RequestInfo tracks the lifecycle of a single request.
type RequestInfo struct {
	ID                    string        `json:"id"`
	Endpoint              string        `json:"endpoint"`
	Model                 string        `json:"model,omitempty"`
	ClientRequestedStream bool          `json:"client_requested_stream"`
	StartTime             time.Time     `json:"start_time"`
	FirstByteTime         *time.Time    `json:"first_byte_time,omitempty"`
	LastActivityTime      time.Time     `json:"last_activity_time"`
	BytesForwarded        int64         `json:"bytes_forwarded"`
	Status                RequestStatus `json:"status,omitempty"`
	Error                 string        `json:"error,omitempty"`
	// Context sizing information
	EstimatedPromptTokens int `json:"estimated_prompt_tokens,omitempty"`
	ChosenCtx              int `json:"chosen_ctx,omitempty"`
	OutputBudgetTokens     int `json:"output_budget_tokens,omitempty"`
	// Actual token counts from Ollama (if available)
	PromptEvalCount int `json:"prompt_eval_count,omitempty"` // Actual input tokens
	EvalCount       int `json:"eval_count,omitempty"`         // Actual output tokens
	// internal: last time a progress event was published (not exported in JSON)
	lastProgressEventTime time.Time
	// internal: whether output limit was exceeded (for warn mode)
	outputLimitExceeded bool
}

// Tracker maintains in-flight and recent request information.
type Tracker struct {
	mu                   sync.RWMutex
	inFlight             map[string]*RequestInfo
	recent               []RequestInfo // circular buffer
	recentHead           int           // index of oldest entry (next to overwrite)
	recentCount          int           // number of entries in buffer
	maxRecent            int
	nextID               int64
	eventBus             *EventBus
	metrics              *Metrics
	calibStore           *calibration.Store
	defaultTokensPerByte float64
	progressInterval     time.Duration
}

// NewTracker creates a new request tracker with the specified maximum recent buffer size.
func NewTracker(maxRecent int, eventBus *EventBus, calibStore *calibration.Store, defaultTokensPerByte float64, progressInterval time.Duration, metrics *Metrics) *Tracker {
	return &Tracker{
		inFlight:             make(map[string]*RequestInfo),
		recent:               make([]RequestInfo, maxRecent), // pre-allocate full buffer
		recentHead:           0,
		recentCount:          0,
		maxRecent:            maxRecent,
		eventBus:             eventBus,
		metrics:              metrics,
		calibStore:           calibStore,
		defaultTokensPerByte: defaultTokensPerByte,
		progressInterval:     progressInterval,
	}
}

// Start registers a new request as in-flight.
func (t *Tracker) Start(reqID string, endpoint string, model string, stream bool) {
	t.mu.Lock()
	now := time.Now()
	req := &RequestInfo{
		ID:                    reqID,
		Endpoint:              endpoint,
		Model:                 model,
		ClientRequestedStream: stream,
		StartTime:             now,
		LastActivityTime:      now,
		BytesForwarded:        0,
		lastProgressEventTime: now,
	}

	t.inFlight[reqID] = req
	inFlightCount := len(t.inFlight)
	t.mu.Unlock()

	// Update metrics
	if t.metrics != nil {
		t.metrics.UpdateInFlight(inFlightCount)
	}

	// Publish event (non-blocking)
	if t.eventBus != nil {
		event := Event{
			Type:      EventRequestStart,
			RequestID: reqID,
			Timestamp: now,
			Endpoint:  endpoint,
			Model:     model,
		}
		t.eventBus.Publish(event)
	}
}

// UpdateModel updates the model for a request (called when model is determined).
func (t *Tracker) UpdateModel(reqID string, model string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if req, exists := t.inFlight[reqID]; exists {
		req.Model = model
	}
}

// UpdateContextData updates context sizing information for a request.
func (t *Tracker) UpdateContextData(reqID string, estimatedPromptTokens, chosenCtx, outputBudgetTokens int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if req, exists := t.inFlight[reqID]; exists {
		req.EstimatedPromptTokens = estimatedPromptTokens
		req.ChosenCtx = chosenCtx
		req.OutputBudgetTokens = outputBudgetTokens
	}
}

// UpdateTokenCounts updates actual token counts from Ollama response.
func (t *Tracker) UpdateTokenCounts(reqID string, promptEvalCount, evalCount int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if req, exists := t.inFlight[reqID]; exists {
		if promptEvalCount > 0 {
			req.PromptEvalCount = promptEvalCount
		}
		if evalCount > 0 {
			req.EvalCount = evalCount
		}
	}
}

// MarkOutputLimitExceeded marks that the output limit was exceeded for a request (warn mode).
func (t *Tracker) MarkOutputLimitExceeded(reqID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if req, exists := t.inFlight[reqID]; exists {
		req.outputLimitExceeded = true
	}
}

// MarkFirstByte marks the first byte time for a request.
func (t *Tracker) MarkFirstByte(reqID string) {
	t.mu.Lock()
	var req *RequestInfo
	var exists bool
	var now time.Time
	if req, exists = t.inFlight[reqID]; exists {
		now = time.Now()
		req.FirstByteTime = &now
		req.LastActivityTime = now
	}
	t.mu.Unlock()

	// Publish event (non-blocking)
	if t.eventBus != nil && exists {
		ttfbMs := int64(now.Sub(req.StartTime).Milliseconds())
		event := Event{
			Type:      EventFirstByte,
			RequestID: reqID,
			Timestamp: now,
			Endpoint:  req.Endpoint,
			Model:     req.Model,
			BytesOut:  req.BytesForwarded,
			EstimatedOutputTokens: EstimateOutputTokens(req.BytesForwarded, req.Model, t.calibStore, t.defaultTokensPerByte),
			TTFBMs:    ttfbMs,
		}
		t.eventBus.Publish(event)
	}
}

// MarkProgress updates the bytes forwarded and last activity time for a request.
func (t *Tracker) MarkProgress(reqID string, bytesDelta int64) {
	t.mu.Lock()
	var req *RequestInfo
	var exists bool
	var now time.Time
	var shouldPublish bool
	if req, exists = t.inFlight[reqID]; exists {
		now = time.Now()
		req.BytesForwarded += bytesDelta
		req.LastActivityTime = now

		// Throttle progress events
		if now.Sub(req.lastProgressEventTime) >= t.progressInterval {
			req.lastProgressEventTime = now
			shouldPublish = true
		}
	}
	t.mu.Unlock()

	// Publish throttled progress event (non-blocking)
	if t.eventBus != nil && exists && shouldPublish {
		var ttfbMs int64
		if req.FirstByteTime != nil {
			ttfbMs = int64(req.FirstByteTime.Sub(req.StartTime).Milliseconds())
		}
		lastActivityAgeMs := int64(now.Sub(req.LastActivityTime).Milliseconds())
		event := Event{
			Type:                 EventProgress,
			RequestID:            reqID,
			Timestamp:            now,
			Endpoint:              req.Endpoint,
			Model:                req.Model,
			BytesOut:             req.BytesForwarded,
			EstimatedOutputTokens: EstimateOutputTokens(req.BytesForwarded, req.Model, t.calibStore, t.defaultTokensPerByte),
			TTFBMs:               ttfbMs,
			LastActivityAgeMs:    lastActivityAgeMs,
		}
		t.eventBus.Publish(event)
	}
}

// GetRequestInfo returns a copy of the request info for the given ID.
// Returns nil if the request is not found.
// This is safe to call concurrently.
func (t *Tracker) GetRequestInfo(reqID string) *RequestInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	req, exists := t.inFlight[reqID]
	if !exists {
		return nil
	}

	// Return a copy to avoid race conditions
	reqCopy := *req
	return &reqCopy
}

// Finish completes a request and moves it to the recent buffer.
func (t *Tracker) Finish(reqID string, status RequestStatus, err error) {
	t.mu.Lock()
	req, exists := t.inFlight[reqID]
	if !exists {
		t.mu.Unlock()
		return
	}

	delete(t.inFlight, reqID)

	// Update final status
	req.Status = status
	if err != nil {
		req.Error = err.Error()
	}

	// Add to recent buffer using O(1) circular buffer
	if t.maxRecent > 0 {
		t.recent[t.recentHead] = *req
		t.recentHead = (t.recentHead + 1) % t.maxRecent
		if t.recentCount < t.maxRecent {
			t.recentCount++
		}
	}
	now := time.Now()
	inFlightCount := len(t.inFlight)
	duration := now.Sub(req.StartTime)
	estimatedTokens := EstimateOutputTokens(req.BytesForwarded, req.Model, t.calibStore, t.defaultTokensPerByte)
	t.mu.Unlock()

	// Record metrics
	if t.metrics != nil {
		t.metrics.RecordRequest(req.Endpoint, req.Model, status, duration, req.BytesForwarded, estimatedTokens)
		t.metrics.UpdateInFlight(inFlightCount)

		// Record timeout metrics
		if status == StatusTimeoutTTFB || status == StatusTimeoutStall || status == StatusTimeoutHard {
			t.metrics.RecordTimeout(status)
		}

		// Record loop detection
		if status == StatusLoopDetected {
			t.metrics.RecordLoopDetected()
		}

		// Record output limit exceeded
		if status == StatusOutputLimitExceeded {
			// Action is determined by config, but we'll use "cancel" as default since that's the only way it gets here
			t.metrics.RecordOutputLimitExceeded("cancel")
		} else if req.outputLimitExceeded {
			// Warn mode - limit was exceeded but request completed normally
			t.metrics.RecordOutputLimitExceeded("warn")
		}
	}

	// Publish completion event (non-blocking)
	if t.eventBus != nil {
		now := time.Now()
		var eventType EventType
		switch status {
		case StatusSuccess:
			eventType = EventDone
		case StatusCanceled:
			eventType = EventCanceled
		case StatusTimeoutTTFB:
			eventType = EventTimeoutTTFB
		case StatusTimeoutStall:
			eventType = EventTimeoutStall
		case StatusTimeoutHard:
			eventType = EventTimeoutHard
		case StatusUpstreamError:
			eventType = EventUpstreamError
		case StatusLoopDetected:
			eventType = EventLoopDetected
		case StatusOutputLimitExceeded:
			eventType = EventOutputLimitExceeded
		default:
			eventType = EventDone
		}

		var ttfbMs int64
		if req.FirstByteTime != nil {
			ttfbMs = int64(req.FirstByteTime.Sub(req.StartTime).Milliseconds())
		}
		lastActivityAgeMs := int64(now.Sub(req.LastActivityTime).Milliseconds())

		event := Event{
			Type:                 eventType,
			RequestID:            reqID,
			Timestamp:            now,
			Endpoint:              req.Endpoint,
			Model:                req.Model,
			BytesOut:             req.BytesForwarded,
			EstimatedOutputTokens: EstimateOutputTokens(req.BytesForwarded, req.Model, t.calibStore, t.defaultTokensPerByte),
			TTFBMs:               ttfbMs,
			LastActivityAgeMs:    lastActivityAgeMs,
			Status:               status,
			Error:                req.Error,
		}
		t.eventBus.Publish(event)
	}
}

// Snapshot returns a copy of current in-flight and recent requests.
// This is safe to call concurrently.
type Snapshot struct {
	InFlight map[string]RequestInfo `json:"in_flight"`
	Recent   []RequestInfo          `json:"recent"`
}

// Snapshot returns a thread-safe snapshot of current tracking state.
func (t *Tracker) Snapshot() Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	snapshot := Snapshot{
		InFlight: make(map[string]RequestInfo, len(t.inFlight)),
		Recent:   make([]RequestInfo, 0, t.recentCount),
	}

	// Copy in-flight requests
	for id, req := range t.inFlight {
		snapshot.InFlight[id] = *req
	}

	// Copy recent requests from circular buffer in order (oldest to newest)
	if t.recentCount > 0 {
		// If buffer is full, start from recentHead (oldest)
		// If not full, start from 0
		start := 0
		if t.recentCount == t.maxRecent {
			start = t.recentHead
		}
		for i := 0; i < t.recentCount; i++ {
			idx := (start + i) % t.maxRecent
			snapshot.Recent = append(snapshot.Recent, t.recent[idx])
		}
	}

	return snapshot
}