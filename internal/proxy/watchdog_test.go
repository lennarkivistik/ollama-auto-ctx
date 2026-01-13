package proxy

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"ollama-auto-ctx/internal/calibration"
	"ollama-auto-ctx/internal/config"
	"ollama-auto-ctx/internal/ollama"
	"ollama-auto-ctx/internal/supervisor"
)

func createTestHandlerWithUpstream(cfg config.Config, upstreamURL string) *Handler {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	features := cfg.Features()

	var tracker *supervisor.Tracker
	var watchdog *supervisor.Watchdog

	if features.Retry || features.Protect {
		tracker = supervisor.NewTracker(cfg.RecentBuffer, nil, nil, 0.25, 250*time.Millisecond, nil)
		if features.Protect {
			watchdog = supervisor.NewWatchdog(tracker, 
				time.Duration(cfg.TimeoutTTFBMs)*time.Millisecond, 
				time.Duration(cfg.TimeoutStallMs)*time.Millisecond, 
				time.Duration(cfg.TimeoutHardMs)*time.Millisecond, 
				logger, nil)
			go watchdog.Run()
		}
	}

	showCache := &ollama.ShowCache{} // mock
	calibStore := &calibration.Store{} // mock

	upstream, _ := url.Parse(upstreamURL)
	handler := NewHandler(cfg, features, upstream, showCache, calibStore, nil, nil, tracker, watchdog, nil, nil, nil, nil, logger)

	return handler
}

func TestWatchdog_TTFBTimeout_Integration(t *testing.T) {
	// Create upstream server that sleeps for a long time (simulates hanging)
	// Use a channel to allow graceful shutdown
	shutdown := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep for longer than the test timeout to simulate hanging
		// But respect both context and shutdown
		select {
		case <-time.After(30 * time.Second):
			// Would respond after 30s, but test should timeout before
		case <-r.Context().Done():
			// Request canceled, exit
			return
		case <-shutdown:
			// Server shutting down
			return
		}
	}))
	defer func() {
		close(shutdown)
		upstream.Close()
	}()

	// Create proxy with watchdog enabled (protect mode)
	cfg := config.Config{
		Mode:          config.ModeProtect,
		TimeoutTTFBMs: 100,
		TimeoutStallMs: 1000,
		TimeoutHardMs:  10000,
		RecentBuffer:  10,
	}

	handler := createTestHandlerWithUpstream(cfg, upstream.URL)
	if handler.watchdog != nil {
		defer handler.watchdog.Shutdown()
	}

	proxy := httptest.NewServer(handler)
	defer proxy.Close()

	// Make request through proxy
	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest("POST", proxy.URL+"/api/chat", strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"test"}]}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
		t.Error("expected request to timeout")
		return
	}

	// Verify error indicates timeout
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected timeout error, got: %v", err)
	}

	// Wait a bit for watchdog to process the timeout (watchdog checks every 1 second)
	time.Sleep(1200 * time.Millisecond)

	// Check tracker has the timed out request
	snapshot := handler.tracker.Snapshot()
	if len(snapshot.Recent) == 0 {
		t.Error("expected timed out request in recent buffer")
		return
	}

	recent := snapshot.Recent[len(snapshot.Recent)-1]
	if recent.Status != supervisor.StatusTimeoutTTFB {
		t.Errorf("expected TTFB timeout status, got: %s", recent.Status)
	}
}

func TestWatchdog_StallTimeout_Integration(t *testing.T) {
	// Create upstream server that writes one chunk then hangs
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"chunk\":1}\n\n")) // Write first chunk
		w.(http.Flusher).Flush()

		// Then hang without more data
		select {}
	}))
	defer upstream.Close()

	// Create proxy with watchdog enabled (protect mode)
	cfg := config.Config{
		Mode:          config.ModeProtect,
		TimeoutTTFBMs: 1000,
		TimeoutStallMs: 100,
		TimeoutHardMs:  10000,
		RecentBuffer:  10,
	}

	handler := createTestHandlerWithUpstream(cfg, upstream.URL)
	if handler.watchdog != nil {
		defer handler.watchdog.Shutdown()
	}

	proxy := httptest.NewServer(handler)
	defer proxy.Close()

	// Make request through proxy
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("POST", proxy.URL+"/api/chat", strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"test"}]}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
		t.Error("expected request to timeout")
		return
	}

	// Verify error indicates timeout
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected timeout error, got: %v", err)
	}

	// Check tracker has the timed out request
	snapshot := handler.tracker.Snapshot()
	if len(snapshot.Recent) == 0 {
		t.Error("expected timed out request in recent buffer")
	}

	recent := snapshot.Recent[len(snapshot.Recent)-1]
	if recent.Status != supervisor.StatusTimeoutStall {
		t.Errorf("expected stall timeout status, got: %s", recent.Status)
	}
}

func TestWatchdog_HardTimeout_Integration(t *testing.T) {
	// Create upstream server that streams slowly but continuously
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		// Stream data continuously but slowly
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for i := 0; i < 50; i++ { // This will take 2.5 seconds
			<-ticker.C
			fmt.Fprintf(w, "data: {\"chunk\":%d}\n\n", i)
			w.(http.Flusher).Flush()
		}
	}))
	defer upstream.Close()

	// Create proxy with watchdog enabled and short hard timeout
	cfg := config.Config{
		Mode:          config.ModeProtect,
		TimeoutTTFBMs: 10000,
		TimeoutStallMs: 10000,
		TimeoutHardMs:  200, // Short hard timeout
		RecentBuffer:  10,
	}

	handler := createTestHandlerWithUpstream(cfg, upstream.URL)
	if handler.watchdog != nil {
		defer handler.watchdog.Shutdown()
	}

	proxy := httptest.NewServer(handler)
	defer proxy.Close()

	// Make request through proxy
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("POST", proxy.URL+"/api/chat", strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"test"}]}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
		t.Error("expected request to timeout")
		return
	}

	// Verify error indicates timeout
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected timeout error, got: %v", err)
	}

	// Check tracker has the timed out request
	snapshot := handler.tracker.Snapshot()
	if len(snapshot.Recent) == 0 {
		t.Error("expected timed out request in recent buffer")
	}

	recent := snapshot.Recent[len(snapshot.Recent)-1]
	if recent.Status != supervisor.StatusTimeoutHard {
		t.Errorf("expected hard timeout status, got: %s", recent.Status)
	}
}

func TestWatchdog_StreamingPreservation(t *testing.T) {
	// Create upstream server that streams NDJSON data
	chunksSent := []string{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`{"chunk":1}`,
			`{"chunk":2}`,
			`{"chunk":3}`,
		}

		for _, chunk := range chunks {
			chunksSent = append(chunksSent, chunk)
			w.Write([]byte(chunk + "\n"))
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond) // Small delay to simulate real streaming
		}
	}))
	defer upstream.Close()

	// Create proxy with watchdog enabled but generous timeouts
	cfg := config.Config{
		Mode:          config.ModeProtect,
		TimeoutTTFBMs: 1000,
		TimeoutStallMs: 1000,
		TimeoutHardMs:  10000,
		RecentBuffer:  10,
	}

	handler := createTestHandlerWithUpstream(cfg, upstream.URL)
	if handler.watchdog != nil {
		defer handler.watchdog.Shutdown()
	}

	proxy := httptest.NewServer(handler)
	defer proxy.Close()

	// Make request through proxy
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("POST", proxy.URL+"/api/chat", strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"test"}]}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// Read all chunks from response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	// Verify we received all chunks
	bodyStr := string(body)
	expectedChunks := []string{
		`{"chunk":1}`,
		`{"chunk":2}`,
		`{"chunk":3}`,
	}

	for _, expected := range expectedChunks {
		if !strings.Contains(bodyStr, expected) {
			t.Errorf("missing chunk in response: %s", expected)
		}
	}

	// Verify request completed successfully (no timeout)
	snapshot := handler.tracker.Snapshot()
	if len(snapshot.Recent) == 0 {
		t.Error("expected completed request in recent buffer")
	}

	recent := snapshot.Recent[len(snapshot.Recent)-1]
	if recent.Status != supervisor.StatusSuccess {
		t.Errorf("expected success status, got: %s", recent.Status)
	}
}

func TestWatchdog_Disabled_NoImpact(t *testing.T) {
	// Create upstream server that responds normally
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"response":"ok"}`))
	}))
	defer upstream.Close()

	// Create proxy with watchdog disabled (off mode)
	cfg := config.Config{
		Mode:         config.ModeOff,
		RecentBuffer: 10,
	}

	handler := createTestHandlerWithUpstream(cfg, upstream.URL)

	proxy := httptest.NewServer(handler)
	defer proxy.Close()

	// Make request through proxy
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("POST", proxy.URL+"/api/chat", strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"test"}]}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// Verify successful response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if string(body) != `{"response":"ok"}` {
		t.Errorf("expected response, got: %s", string(body))
	}
}
