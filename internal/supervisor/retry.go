package supervisor

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"
)

// RetryConfig holds configuration for retry logic.
type RetryConfig struct {
	Enabled           bool          // SUPERVISOR_RETRY_ENABLED
	MaxAttempts       int           // SUPERVISOR_RETRY_MAX_ATTEMPTS (default 2)
	Backoff           time.Duration // SUPERVISOR_RETRY_BACKOFF (default 250ms)
	OnlyNonStreaming  bool          // SUPERVISOR_RETRY_ONLY_NON_STREAMING (default true)
	MaxResponseBytes  int64         // SUPERVISOR_RETRY_MAX_RESPONSE_BYTES (default 8MB)
}

// RetryResult represents the outcome of a retried request.
type RetryResult struct {
	Response   *http.Response
	Body       []byte // buffered response body (for non-streaming)
	Attempts   int
	LastError  error
	TooLarge   bool   // response exceeded MaxResponseBytes
}

// Retryer handles retry logic for non-streaming requests.
type Retryer struct {
	cfg    RetryConfig
	client *http.Client
}

// NewRetryer creates a new Retryer with the given configuration.
func NewRetryer(cfg RetryConfig) *Retryer {
	return &Retryer{
		cfg: cfg,
		client: &http.Client{
			// Don't set timeout here - let context handle it
			Transport: http.DefaultTransport,
		},
	}
}

// ShouldRetry determines if a response/error warrants a retry.
func ShouldRetry(resp *http.Response, err error) bool {
	// Retry on connection errors
	if err != nil {
		return true
	}
	// Retry on 5xx errors
	if resp != nil && resp.StatusCode >= 500 {
		return true
	}
	return false
}

// IsEligible checks if a request is eligible for retry.
// Only non-streaming requests to /api/chat and /api/generate are eligible.
func (r *Retryer) IsEligible(req *http.Request, clientStream bool, endpoint string) bool {
	if !r.cfg.Enabled {
		return false
	}
	// Only retry non-streaming by default
	if r.cfg.OnlyNonStreaming && clientStream {
		return false
	}
	// Only retry the two core endpoints
	if endpoint != "chat" && endpoint != "generate" {
		return false
	}
	return true
}

// DoWithRetry executes a request with retry logic.
// The requestBody should be the complete body bytes to send.
// Returns the result including buffered response body on success.
func (r *Retryer) DoWithRetry(ctx context.Context, upstreamURL string, method string, requestBody []byte, headers http.Header) RetryResult {
	result := RetryResult{}

	for attempt := 1; attempt <= r.cfg.MaxAttempts; attempt++ {
		result.Attempts = attempt

		// Create fresh request for each attempt
		req, err := http.NewRequestWithContext(ctx, method, upstreamURL, bytes.NewReader(requestBody))
		if err != nil {
			result.LastError = err
			continue
		}

		// Copy headers
		for k, v := range headers {
			req.Header[k] = v
		}
		req.ContentLength = int64(len(requestBody))

		// Execute request
		resp, err := r.client.Do(req)
		if err != nil {
			result.LastError = err
			// Check if context was canceled (don't retry)
			if ctx.Err() != nil {
				return result
			}
			// Wait before retry
			if attempt < r.cfg.MaxAttempts {
				select {
				case <-ctx.Done():
					return result
				case <-time.After(r.cfg.Backoff):
				}
			}
			continue
		}

		// Check if we should retry based on response
		if ShouldRetry(resp, nil) && attempt < r.cfg.MaxAttempts {
			_ = resp.Body.Close()
			result.LastError = nil
			// Wait before retry
			select {
			case <-ctx.Done():
				return result
			case <-time.After(r.cfg.Backoff):
			}
			continue
		}

		// Success or final attempt - read response body
		result.Response = resp
		result.LastError = nil

		// Read body with size limit
		body, err := readBodyWithLimit(resp.Body, r.cfg.MaxResponseBytes)
		_ = resp.Body.Close()
		if err != nil {
			result.LastError = err
			result.TooLarge = true
			return result
		}
		result.Body = body
		return result
	}

	return result
}

// readBodyWithLimit reads up to maxBytes from the reader.
// Returns error if body exceeds limit.
func readBodyWithLimit(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return io.ReadAll(r)
	}

	buf := &bytes.Buffer{}
	limited := io.LimitReader(r, maxBytes+1)
	n, err := buf.ReadFrom(limited)
	if err != nil {
		return nil, err
	}
	if n > maxBytes {
		return nil, io.ErrUnexpectedEOF // signals "too large"
	}
	return buf.Bytes(), nil
}
