package proxy

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"ollama-auto-ctx/internal/calibration"
)

func TestTapReadCloser_OutputLimit_Cancel(t *testing.T) {
	// Create a reader that produces enough data to exceed limit
	// At 0.25 tokens/byte, 1000 tokens = 4000 bytes
	// We need more than 4000 bytes to exceed the limit
	data := strings.Repeat("This is test data. ", 500) // ~10KB, ~2500 tokens
	rc := io.NopCloser(strings.NewReader(data))

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	calibStore := calibration.NewStore(0.20, calibration.Params{TokensPerByte: 0.25}, "")

	_, cancel := context.WithCancel(context.Background())
	cancelCalled := false
	cancelFunc := func() {
		cancelCalled = true
		cancel()
	}

	// Set limit to 1000 tokens
	// With ~2500 tokens from data, we should exceed this
	outputTokenLimit := int64(1000)
	minOutputBytes := int64(256) // Low threshold for testing

	tap := NewTapReadCloser(
		rc,
		"application/json",
		0,
		1024*1024,
		calibration.Sample{Model: "test"},
		calibStore,
		nil, // no tracker for this test
		nil, // no loop detector
		"test-req",
		logger,
		outputTokenLimit,
		"cancel",
		cancelFunc,
		minOutputBytes,
		nil, // no data store for this test
	).(*TapReadCloser)

	// Read data - should trigger limit after minimum threshold
	buf := make([]byte, 1024)
	totalRead := 0
	limitTriggered := false
	for {
		n, err := tap.Read(buf)
		totalRead += n
		if err == io.EOF {
			// If we hit EOF, check if limit was exceeded
			if tap.limitExceeded {
				limitTriggered = true
			}
			break
		}
		if err != nil {
			// Expected when limit is exceeded and canceled
			limitTriggered = true
			break
		}
		if totalRead > len(data) {
			break
		}
		// Check if limit was exceeded
		if tap.limitExceeded {
			limitTriggered = true
		}
	}

	// Verify limit was triggered
	if !limitTriggered {
		t.Errorf("expected limit to be exceeded (read %d bytes, limit %d tokens ≈ %d bytes)", 
			totalRead, outputTokenLimit, outputTokenLimit*4)
	}

	// Verify cancel was called
	if !cancelCalled {
		t.Error("expected cancel function to be called when limit exceeded")
	}
}

func TestTapReadCloser_OutputLimit_Warn(t *testing.T) {
	// Create a reader that produces enough data to exceed limit
	// At 0.25 tokens/byte, 1000 tokens = 4000 bytes
	data := strings.Repeat("This is test data. ", 500) // ~10KB, ~2500 tokens
	rc := io.NopCloser(strings.NewReader(data))

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	calibStore := calibration.NewStore(0.20, calibration.Params{TokensPerByte: 0.25}, "")

	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelCalled := false
	cancelFunc := func() {
		cancelCalled = true
		cancel()
	}

	// Set limit to 1000 tokens
	outputTokenLimit := int64(1000)
	minOutputBytes := int64(256) // Low threshold for testing

	tap := NewTapReadCloser(
		rc,
		"application/json",
		0,
		1024*1024,
		calibration.Sample{Model: "test"},
		calibStore,
		nil,
		nil,
		"test-req",
		logger,
		outputTokenLimit,
		"warn", // Warn only, don't cancel
		cancelFunc,
		minOutputBytes,
		nil, // no data store for this test
	).(*TapReadCloser)

	// Read all data - should warn but not cancel
	buf := make([]byte, 1024)
	totalRead := 0
	for {
		n, err := tap.Read(buf)
		totalRead += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if totalRead >= len(data) {
			break
		}
	}

	// Verify cancel was NOT called (warn mode)
	if cancelCalled {
		t.Error("expected cancel function NOT to be called in warn mode")
	}

	// Verify limit was exceeded
	if !tap.limitExceeded {
		t.Errorf("expected limit to be exceeded (read %d bytes, limit %d tokens ≈ %d bytes)", 
			totalRead, outputTokenLimit, outputTokenLimit*4)
	}
}

func TestTapReadCloser_OutputLimit_Disabled(t *testing.T) {
	data := strings.Repeat("test data ", 1000)
	rc := io.NopCloser(strings.NewReader(data))

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	calibStore := calibration.NewStore(0.20, calibration.Params{TokensPerByte: 0.25}, "")

	tap := NewTapReadCloser(
		rc,
		"application/json",
		0,
		1024*1024,
		calibration.Sample{Model: "test"},
		calibStore,
		nil,
		nil,
		"test-req",
		logger,
		0, // Disabled (0 = no limit)
		"cancel",
		nil,
		256,
		nil, // no data store for this test
	).(*TapReadCloser)

	// Read all data - should not trigger limit
	buf := make([]byte, 1024)
	for {
		_, err := tap.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Verify limit was not exceeded (because it's disabled)
	if tap.limitExceeded {
		t.Error("expected limit not to be exceeded when disabled")
	}
}

func TestTapReadCloser_OutputLimit_BelowMinimum(t *testing.T) {
	// Small amount of data, below minimum threshold
	data := "small data"
	rc := io.NopCloser(strings.NewReader(data))

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	calibStore := calibration.NewStore(0.20, calibration.Params{TokensPerByte: 0.25}, "")

	cancelCalled := false
	cancelFunc := func() {
		cancelCalled = true
	}

	outputTokenLimit := int64(1) // Very low limit
	minOutputBytes := int64(1000) // High minimum - data is below this

	tap := NewTapReadCloser(
		rc,
		"application/json",
		0,
		1024*1024,
		calibration.Sample{Model: "test"},
		calibStore,
		nil,
		nil,
		"test-req",
		logger,
		outputTokenLimit,
		"cancel",
		cancelFunc,
		minOutputBytes,
		nil, // no data store for this test
	).(*TapReadCloser)

	// Read all data
	buf := make([]byte, 1024)
	for {
		_, err := tap.Read(buf)
		if err == io.EOF {
			break
		}
	}

	// Verify cancel was NOT called (below minimum threshold)
	if cancelCalled {
		t.Error("expected cancel function NOT to be called when below minimum output threshold")
	}
}
