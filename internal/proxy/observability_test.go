package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"ollama-auto-ctx/internal/calibration"
	"ollama-auto-ctx/internal/config"
	"ollama-auto-ctx/internal/ollama"
	"ollama-auto-ctx/internal/supervisor"
)

func createTestHandlerWithObs(cfg config.Config) *Handler {
	return createTestHandlerWithObsAndCleanup(cfg, nil)
}

func createTestHandlerWithObsAndCleanup(cfg config.Config, cleanup func(*supervisor.EventBus)) *Handler {
	logger := slog.Default()

	var tracker *supervisor.Tracker
	var watchdog *supervisor.Watchdog
	var eventBus *supervisor.EventBus

	if cfg.SupervisorEnabled {
		if cfg.SupervisorObsEnabled {
			eventBus = supervisor.NewEventBus(100)
			if cleanup != nil {
				cleanup(eventBus)
			}
		}

		defaults := calibration.Params{
			TokensPerByte: cfg.DefaultTokensPerByte,
		}
		calibStore := calibration.NewStore(0.20, defaults, "")

		tracker = supervisor.NewTracker(cfg.SupervisorRecentBuffer, eventBus, calibStore, cfg.DefaultTokensPerByte, cfg.SupervisorObsProgressInterval, nil)

		if cfg.SupervisorWatchdogEnabled {
			watchdog = supervisor.NewWatchdog(tracker, cfg.SupervisorTTFBTimeout, cfg.SupervisorStallTimeout, cfg.SupervisorHardTimeout, logger, nil)
			go watchdog.Run()
		}
	}

	showCache := &ollama.ShowCache{}
	calibStore := &calibration.Store{}

	handler := NewHandler(cfg, &url.URL{Scheme: "http", Host: "localhost"}, showCache, calibStore, tracker, watchdog, eventBus, nil, nil, nil, logger)

	return handler
}

func TestDebugRequestsEndpoint(t *testing.T) {
	cfg := config.Config{
		SupervisorEnabled:       true,
		SupervisorTrackRequests: true,
		SupervisorObsEnabled:    true,
		SupervisorObsRequestsEndpoint: true,
		SupervisorRecentBuffer:  10,
		DefaultTokensPerByte:    0.25,
		SupervisorObsProgressInterval: 250 * time.Millisecond,
	}

	handler := createTestHandlerWithObs(cfg)

	// Start a request
	handler.tracker.Start("req1", "/api/chat", "llama2", false)
	handler.tracker.MarkProgress("req1", 100)
	handler.tracker.Finish("req1", supervisor.StatusSuccess, nil)

	req := httptest.NewRequest("GET", "/debug/requests", nil)
	w := httptest.NewRecorder()

	handler.handleDebugRequests(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response struct {
		InFlight map[string]struct {
			supervisor.RequestInfo
			EstimatedOutputTokens int64 `json:"estimated_output_tokens"`
		} `json:"in_flight"`
		Recent []struct {
			supervisor.RequestInfo
			EstimatedOutputTokens int64 `json:"estimated_output_tokens"`
		} `json:"recent"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(response.Recent) == 0 {
		t.Error("expected at least one recent request")
	}

	// Check that estimated tokens are included
	if response.Recent[0].EstimatedOutputTokens == 0 && response.Recent[0].RequestInfo.BytesForwarded > 0 {
		t.Error("expected estimated output tokens to be calculated")
	}
}

func TestDebugRequestsEndpoint_Disabled(t *testing.T) {
	cfg := config.Config{
		SupervisorEnabled: false,
	}

	handler := createTestHandlerWithObs(cfg)

	req := httptest.NewRequest("GET", "/debug/requests", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should not handle the endpoint when disabled
	if w.Code == http.StatusOK {
		t.Error("endpoint should not be available when disabled")
	}
}

func TestSSEEndpoint_WithRequestLifecycle(t *testing.T) {
	cfg := config.Config{
		SupervisorEnabled:       true,
		SupervisorTrackRequests: true,
		SupervisorObsEnabled:    true,
		SupervisorObsSSEEndpoint: true,
		SupervisorRecentBuffer:  10,
		DefaultTokensPerByte:    0.25,
		SupervisorObsProgressInterval: 50 * time.Millisecond, // Fast for testing
	}

	var eventBus *supervisor.EventBus
	handler := createTestHandlerWithObsAndCleanup(cfg, func(eb *supervisor.EventBus) {
		eventBus = eb
	})
	defer func() {
		if eventBus != nil {
			eventBus.Shutdown()
		}
	}()

	// Create a request with a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "/events", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	// Start SSE handler in goroutine
	done := make(chan bool)
	var events []string
	go func() {
		handler.handleSSEEvents(w, req)
		done <- true
	}()

	// Give handler time to set up
	time.Sleep(10 * time.Millisecond)

	// Start a request to generate events
	handler.tracker.Start("test-req", "/api/chat", "llama2", false)
	time.Sleep(20 * time.Millisecond)

	handler.tracker.MarkFirstByte("test-req")
	time.Sleep(20 * time.Millisecond)

	handler.tracker.MarkProgress("test-req", 100)
	time.Sleep(100 * time.Millisecond) // Wait for throttled progress event

	handler.tracker.Finish("test-req", supervisor.StatusSuccess, nil)
	time.Sleep(20 * time.Millisecond)

	// Cancel context to stop SSE handler
	cancel()

	// Wait for handler to finish (with timeout)
	select {
	case <-done:
		// Handler finished
	case <-time.After(1 * time.Second):
		t.Error("SSE handler did not finish in time")
	}

	// Read events from response
	scanner := bufio.NewScanner(w.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			events = append(events, line)
		}
		if len(events) >= 5 { // We expect at least a few events
			break
		}
	}

	if len(events) == 0 {
		t.Error("expected to receive at least one SSE event")
	}

	// Verify we got request_start event
	foundStart := false
	for _, event := range events {
		if strings.Contains(event, "request_start") {
			foundStart = true
			break
		}
	}
	if !foundStart {
		t.Error("expected to receive request_start event")
	}
}

func TestSSEEndpoint_SlowConsumer(t *testing.T) {
	cfg := config.Config{
		SupervisorEnabled:       true,
		SupervisorTrackRequests: true,
		SupervisorObsEnabled:    true,
		SupervisorObsSSEEndpoint: true,
		SupervisorRecentBuffer:  10,
		DefaultTokensPerByte:    0.25,
		SupervisorObsProgressInterval: 10 * time.Millisecond,
	}

	var eventBus *supervisor.EventBus
	handler := createTestHandlerWithObsAndCleanup(cfg, func(eb *supervisor.EventBus) {
		eventBus = eb
	})
	defer func() {
		if eventBus != nil {
			eventBus.Shutdown()
		}
	}()

	// Create a request with a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "/events", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	// Start SSE handler
	done := make(chan bool)
	go func() {
		handler.handleSSEEvents(w, req)
		done <- true
	}()

	// Give handler time to set up
	time.Sleep(10 * time.Millisecond)

	// Publish many events rapidly
	for i := 0; i < 50; i++ {
		reqID := "req" + strconv.Itoa(i)
		handler.tracker.Start(reqID, "/api/chat", "model", false)
		handler.tracker.MarkProgress(reqID, 100)
		handler.tracker.Finish(reqID, supervisor.StatusSuccess, nil)
	}

	// Handler should not block - verify by checking that we can still make requests
	time.Sleep(50 * time.Millisecond)

	// The fact that we got here without blocking is the test
	// In a real scenario, slow consumers would cause events to be dropped (fail-open)
}