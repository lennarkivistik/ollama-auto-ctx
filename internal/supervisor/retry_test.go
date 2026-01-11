package supervisor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRetryer_IsEligible(t *testing.T) {
	tests := []struct {
		name          string
		cfg           RetryConfig
		clientStream  bool
		endpoint      string
		wantEligible  bool
	}{
		{
			name: "disabled retryer",
			cfg: RetryConfig{
				Enabled: false,
			},
			clientStream: false,
			endpoint:     "chat",
			wantEligible: false,
		},
		{
			name: "non-streaming chat",
			cfg: RetryConfig{
				Enabled:          true,
				OnlyNonStreaming: true,
			},
			clientStream: false,
			endpoint:     "chat",
			wantEligible: true,
		},
		{
			name: "streaming chat with only-non-streaming",
			cfg: RetryConfig{
				Enabled:          true,
				OnlyNonStreaming: true,
			},
			clientStream: true,
			endpoint:     "chat",
			wantEligible: false,
		},
		{
			name: "streaming chat without only-non-streaming",
			cfg: RetryConfig{
				Enabled:          true,
				OnlyNonStreaming: false,
			},
			clientStream: true,
			endpoint:     "chat",
			wantEligible: true,
		},
		{
			name: "non-streaming generate",
			cfg: RetryConfig{
				Enabled:          true,
				OnlyNonStreaming: true,
			},
			clientStream: false,
			endpoint:     "generate",
			wantEligible: true,
		},
		{
			name: "unknown endpoint",
			cfg: RetryConfig{
				Enabled:          true,
				OnlyNonStreaming: true,
			},
			clientStream: false,
			endpoint:     "unknown",
			wantEligible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryer := NewRetryer(tt.cfg)
			got := retryer.IsEligible(nil, tt.clientStream, tt.endpoint)
			if got != tt.wantEligible {
				t.Errorf("IsEligible() = %v, want %v", got, tt.wantEligible)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	tests := []struct {
		name     string
		resp     *http.Response
		err      error
		expected bool
	}{
		{
			name:     "connection error",
			resp:     nil,
			err:      context.DeadlineExceeded,
			expected: true,
		},
		{
			name: "500 error",
			resp: &http.Response{
				StatusCode: 500,
			},
			err:      nil,
			expected: true,
		},
		{
			name: "502 error",
			resp: &http.Response{
				StatusCode: 502,
			},
			err:      nil,
			expected: true,
		},
		{
			name: "200 success",
			resp: &http.Response{
				StatusCode: 200,
			},
			err:      nil,
			expected: false,
		},
		{
			name: "400 client error",
			resp: &http.Response{
				StatusCode: 400,
			},
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldRetry(tt.resp, tt.err)
			if got != tt.expected {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRetryer_DoWithRetry_Success(t *testing.T) {
	// Server that always succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	cfg := RetryConfig{
		Enabled:          true,
		MaxAttempts:      3,
		Backoff:          10 * time.Millisecond,
		MaxResponseBytes: 1024,
	}

	retryer := NewRetryer(cfg)
	result := retryer.DoWithRetry(
		context.Background(),
		server.URL,
		http.MethodPost,
		[]byte(`{"test": true}`),
		http.Header{"Content-Type": []string{"application/json"}},
	)

	if result.LastError != nil {
		t.Errorf("unexpected error: %v", result.LastError)
	}
	if result.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", result.Attempts)
	}
	if result.Response.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.Response.StatusCode)
	}
}

func TestRetryer_DoWithRetry_RetryOn500(t *testing.T) {
	attempts := 0
	// Server that fails twice then succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	cfg := RetryConfig{
		Enabled:          true,
		MaxAttempts:      3,
		Backoff:          10 * time.Millisecond,
		MaxResponseBytes: 1024,
	}

	retryer := NewRetryer(cfg)
	result := retryer.DoWithRetry(
		context.Background(),
		server.URL,
		http.MethodPost,
		[]byte(`{"test": true}`),
		http.Header{"Content-Type": []string{"application/json"}},
	)

	if result.LastError != nil {
		t.Errorf("unexpected error: %v", result.LastError)
	}
	if result.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", result.Attempts)
	}
	if result.Response.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.Response.StatusCode)
	}
}

func TestRetryer_DoWithRetry_MaxAttemptsExceeded(t *testing.T) {
	// Server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := RetryConfig{
		Enabled:          true,
		MaxAttempts:      2,
		Backoff:          10 * time.Millisecond,
		MaxResponseBytes: 1024,
	}

	retryer := NewRetryer(cfg)
	result := retryer.DoWithRetry(
		context.Background(),
		server.URL,
		http.MethodPost,
		[]byte(`{"test": true}`),
		http.Header{"Content-Type": []string{"application/json"}},
	)

	if result.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", result.Attempts)
	}
	// Final response should be the 500
	if result.Response.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", result.Response.StatusCode)
	}
}

func TestRetryer_DoWithRetry_ContextCanceled(t *testing.T) {
	// Server that sleeps
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := RetryConfig{
		Enabled:          true,
		MaxAttempts:      3,
		Backoff:          10 * time.Millisecond,
		MaxResponseBytes: 1024,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	retryer := NewRetryer(cfg)
	result := retryer.DoWithRetry(
		ctx,
		server.URL,
		http.MethodPost,
		[]byte(`{"test": true}`),
		http.Header{"Content-Type": []string{"application/json"}},
	)

	if result.LastError == nil {
		t.Errorf("expected context canceled error")
	}
	if result.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", result.Attempts)
	}
}

func TestRetryer_DoWithRetry_ResponseTooLarge(t *testing.T) {
	// Server that returns large response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write more than max bytes
		for i := 0; i < 100; i++ {
			w.Write([]byte("This is a large response body. "))
		}
	}))
	defer server.Close()

	cfg := RetryConfig{
		Enabled:          true,
		MaxAttempts:      1,
		MaxResponseBytes: 100, // Small limit
	}

	retryer := NewRetryer(cfg)
	result := retryer.DoWithRetry(
		context.Background(),
		server.URL,
		http.MethodPost,
		[]byte(`{"test": true}`),
		http.Header{"Content-Type": []string{"application/json"}},
	)

	if !result.TooLarge {
		t.Errorf("expected TooLarge flag to be set")
	}
}
