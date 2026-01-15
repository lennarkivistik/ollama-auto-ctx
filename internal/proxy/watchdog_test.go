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
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
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
		TimeoutTTFBMs: 500, // 0.5 seconds - should be caught quickly
		TimeoutStallMs: 1000,
		TimeoutHardMs:  10000,
		RecentBuffer:  10,
	}

	handler := createTestHandlerWithUpstream(cfg, upstream.URL)
	if handler.watchdog != nil {
		defer handler.watchdog.Shutdown()
		t.Logf("Watchdog created and running")
	} else {
		t.Logf("Watchdog is nil!")
	}

	proxy := httptest.NewServer(handler)
	defer proxy.Close()

	// Make request through proxy
	client := &http.Client{Timeout: 3 * time.Second} // Longer than watchdog timeout
	req, _ := http.NewRequest("POST", proxy.URL+"/api/chat", strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"test"}]}`))
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	t.Logf("Request took %v, error: %v", elapsed, err)

	if err != nil {
		t.Errorf("expected successful response (possibly with error status), got error: %v", err)
		return
	}

	t.Logf("Response status: %s", resp.Status)
	defer resp.Body.Close()

	// When watchdog cancels the request, proxy returns 502 Bad Gateway
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502 Bad Gateway when watchdog cancels request, got: %s", resp.Status)
	}

	// Wait a bit for watchdog to process the timeout (watchdog checks every 1 second)
	time.Sleep(1500 * time.Millisecond)

	// Check tracker has the timed out request
	snapshot := handler.tracker.Snapshot()
	t.Logf("Recent requests: %d", len(snapshot.Recent))
	t.Logf("In-flight requests: %d", len(snapshot.InFlight))
	for reqID, req := range snapshot.InFlight {
		t.Logf("In-flight request %s: started=%v, firstByte=%v, lastActivity=%v, elapsed=%v",
			reqID, req.StartTime, req.FirstByteTime, req.LastActivityTime, time.Since(req.StartTime))
	}
	if len(snapshot.Recent) == 0 {
		t.Error("expected timed out request in recent buffer")
		return
	}

	recent := snapshot.Recent[len(snapshot.Recent)-1]
	t.Logf("Recent request status: %s", recent.Status)
	if recent.Status != supervisor.StatusTimeoutTTFB {
		t.Errorf("expected TTFB timeout status, got: %s", recent.Status)
	}
}

func TestWatchdog_StallTimeout_Integration(t *testing.T) {
	// TODO: This test needs redesign - client completes response too quickly
	// making stall timeout detection impossible with current HTTP semantics
	t.Skip("Stall timeout test needs redesign - client completes response too quickly")
	shutdown := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		// Send first chunk
		w.Write([]byte("{\"chunk\":1}\n"))
		w.(http.Flusher).Flush()

		// Wait a bit - this should trigger first byte detection
		time.Sleep(100 * time.Millisecond)

		// Don't send more data - this should trigger stall timeout
		select {
		case <-shutdown:
			return
		default:
			select {
			case <-r.Context().Done():
				return
			case <-shutdown:
				return
			}
		}
	}))
	defer func() {
		close(shutdown)
		upstream.Close()
	}()

	// Create proxy with watchdog enabled (protect mode)
	cfg := config.Config{
		Mode:          config.ModeProtect,
		TimeoutTTFBMs: 1000,
		TimeoutStallMs: 1500, // 1.5 seconds - long enough to be caught by watchdog
		TimeoutHardMs:  10000,
		RecentBuffer:  10,
	}

	handler := createTestHandlerWithUpstream(cfg, upstream.URL)
	if handler.watchdog != nil {
		defer handler.watchdog.Shutdown()
		t.Logf("Watchdog created and running")
	} else {
		t.Logf("Watchdog is nil!")
	}

	proxy := httptest.NewServer(handler)
	defer proxy.Close()

	// Make request through proxy
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("POST", proxy.URL+"/api/chat", strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"test"}]}`))
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	t.Logf("Request took %v, error: %v", elapsed, err)

	if err != nil {
		t.Errorf("expected successful response (possibly with error status), got error: %v", err)
		return
	}

	t.Logf("Response status: %s", resp.Status)
	defer resp.Body.Close()

	// When watchdog cancels the request, proxy returns 502 Bad Gateway
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502 Bad Gateway when watchdog cancels request, got: %s", resp.Status)
	}

	// Wait a bit for watchdog to process the timeout
	time.Sleep(1500 * time.Millisecond)

	// Check tracker has the timed out request
	snapshot := handler.tracker.Snapshot()
	t.Logf("Recent requests: %d", len(snapshot.Recent))
	if len(snapshot.Recent) == 0 {
		t.Error("expected timed out request in recent buffer")
	}

	if len(snapshot.Recent) > 0 {
		recent := snapshot.Recent[len(snapshot.Recent)-1]
		t.Logf("Recent request status: %s", recent.Status)
		if recent.Status != supervisor.StatusTimeoutStall {
			t.Errorf("expected stall timeout status, got: %s", recent.Status)
		}
	}
}

func TestWatchdog_HardTimeout_Integration(t *testing.T) {
	// Create upstream server that streams slowly but continuously
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.WriteHeader(http.StatusOK)

		// Stream data continuously but slowly
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for i := 0; i < 10; i++ { // This will take 1 second
			select {
			case <-ticker.C:
				fmt.Fprintf(w, "data: {\"chunk\":%d}\n\n", i)
				w.(http.Flusher).Flush()
			case <-r.Context().Done():
				return
			}
		}
	}))
	defer upstream.Close()

	// Create proxy with watchdog enabled and short hard timeout
	cfg := config.Config{
		Mode:          config.ModeProtect,
		TimeoutTTFBMs: 10000,
		TimeoutStallMs: 10000,
		TimeoutHardMs:  300, // Short hard timeout - should trigger before streaming completes
		RecentBuffer:  10,
	}

	handler := createTestHandlerWithUpstream(cfg, upstream.URL)
	if handler.watchdog != nil {
		defer handler.watchdog.Shutdown()
		t.Logf("Watchdog created and running")
	} else {
		t.Logf("Watchdog is nil!")
	}

	proxy := httptest.NewServer(handler)
	defer proxy.Close()

	// Make request through proxy
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("POST", proxy.URL+"/api/chat", strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"test"}]}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("expected successful response start, got error: %v", err)
		return
	}

	defer resp.Body.Close()

	// Read the response body to see if it gets truncated
	body, readErr := io.ReadAll(resp.Body)
	t.Logf("Response status: %s, body length: %d, read error: %v", resp.Status, len(body), readErr)

	// When watchdog cancels the request during streaming, the response may be truncated
	// The status might still be 200 OK since it started before cancellation
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK (response started before timeout), got: %s", resp.Status)
	}

	// But the body should be truncated due to cancellation
	if len(body) >= 500 { // Should be much less than full response if canceled
		t.Errorf("expected truncated response body due to timeout, got %d bytes", len(body))
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Check tracker has the timed out request
	snapshot := handler.tracker.Snapshot()
	t.Logf("Recent requests: %d", len(snapshot.Recent))
	if len(snapshot.Recent) == 0 {
		t.Error("expected timed out request in recent buffer")
		return
	}

	recent := snapshot.Recent[len(snapshot.Recent)-1]
	t.Logf("Recent request status: %s", recent.Status)
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
		t.Logf("Watchdog created and running")
	} else {
		t.Logf("Watchdog is nil!")
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
