package supervisor

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestHealthChecker_Healthy(t *testing.T) {
	// Create a test server that responds successfully
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"models":[]}`))
		}
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	hc := NewHealthChecker(server.URL, 100*time.Millisecond, 5*time.Second, nil, logger)
	defer hc.Shutdown()

	// Wait for initial check
	time.Sleep(150 * time.Millisecond)

	if !hc.Healthy() {
		t.Error("expected health checker to be healthy")
	}

	if hc.LastError() != "" {
		t.Errorf("expected no error, got: %s", hc.LastError())
	}
}

func TestHealthChecker_Unhealthy(t *testing.T) {
	// Create a test server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	hc := NewHealthChecker(server.URL, 100*time.Millisecond, 5*time.Second, nil, logger)
	defer hc.Shutdown()

	// Wait for initial check
	time.Sleep(150 * time.Millisecond)

	if hc.Healthy() {
		t.Error("expected health checker to be unhealthy")
	}

	if hc.LastError() == "" {
		t.Error("expected error message")
	}
}

func TestHealthChecker_ConnectionError(t *testing.T) {
	// Use invalid URL to cause connection error
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	hc := NewHealthChecker("http://localhost:99999", 100*time.Millisecond, 1*time.Second, nil, logger)
	defer hc.Shutdown()

	// Wait for initial check
	time.Sleep(150 * time.Millisecond)

	if hc.Healthy() {
		t.Error("expected health checker to be unhealthy on connection error")
	}

	if hc.LastError() == "" {
		t.Error("expected error message for connection failure")
	}
}

func TestHealthChecker_Shutdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	hc := NewHealthChecker(server.URL, 100*time.Millisecond, 5*time.Second, nil, logger)

	// Shutdown should not panic
	hc.Shutdown()

	// Wait a bit to ensure goroutine stops
	time.Sleep(200 * time.Millisecond)
}
