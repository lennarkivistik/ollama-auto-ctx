package proxy

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"ollama-auto-ctx/internal/config"
	"ollama-auto-ctx/internal/storage"
	"ollama-auto-ctx/internal/supervisor"
)

func TestChooseFinalCtx(t *testing.T) {
	desired := 8192
	hardMax := 16384

	// No user value: always set.
	ctx, override, clamped := chooseFinalCtx(desired, hardMax, 0, false, config.OverrideIfTooSmall)
	if ctx != desired || !override || clamped {
		t.Fatalf("expected ctx=%d override=true clamped=false, got ctx=%d override=%v clamped=%v", desired, ctx, override, clamped)
	}

	// User ctx smaller, policy if_too_small -> increase.
	ctx, override, clamped = chooseFinalCtx(desired, hardMax, 4096, true, config.OverrideIfTooSmall)
	if ctx != desired || !override || clamped {
		t.Fatalf("expected ctx=%d override=true clamped=false, got ctx=%d override=%v clamped=%v", desired, ctx, override, clamped)
	}

	// User ctx larger, policy if_too_small -> keep user ctx.
	ctx, override, clamped = chooseFinalCtx(desired, hardMax, 12288, true, config.OverrideIfTooSmall)
	if ctx != 12288 || override || clamped {
		t.Fatalf("expected ctx=12288 override=false clamped=false, got ctx=%d override=%v clamped=%v", ctx, override, clamped)
	}

	// User ctx larger than hardMax -> clamp down.
	ctx, override, clamped = chooseFinalCtx(desired, 8192, 16384, true, config.OverrideIfTooSmall)
	if ctx != 8192 || !override || !clamped {
		t.Fatalf("expected ctx=8192 override=true clamped=true, got ctx=%d override=%v clamped=%v", ctx, override, clamped)
	}

	// Policy always overrides user ctx.
	ctx, override, clamped = chooseFinalCtx(desired, hardMax, 4096, true, config.OverrideAlways)
	if ctx != desired || !override || clamped {
		t.Fatalf("expected ctx=%d override=true clamped=false, got ctx=%d override=%v clamped=%v", desired, ctx, override, clamped)
	}

	// Policy if_missing leaves user ctx unchanged.
	ctx, override, clamped = chooseFinalCtx(desired, hardMax, 4096, true, config.OverrideIfMissing)
	if ctx != 4096 || override || clamped {
		t.Fatalf("expected ctx=4096 override=false clamped=false, got ctx=%d override=%v clamped=%v", ctx, override, clamped)
	}
}

func TestFinalizeStorageFromTracker_TTFBPreservation(t *testing.T) {
	// Create a mock storage that records what gets saved
	var savedTTFB *int
	mockStorage := &mockStore{
		updateFunc: func(id string, upd storage.RequestUpdate) {
			savedTTFB = upd.TTFBMs
		},
	}

	// Create a tracker and start a request
	tracker := supervisor.NewTracker(10, nil, nil, 0.25, 250*time.Millisecond, nil)
	tracker.Start("test-req", "/api/chat", "llama2", true)

	// Simulate some time passing before first byte (like network latency)
	time.Sleep(10 * time.Millisecond)

	// Mark first byte
	tracker.MarkFirstByte("test-req")

	// Create handler with mock storage and logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := &Handler{
		store:   mockStorage,
		tracker: tracker,
		logger:  logger,
	}

	startTime := time.Now().Add(-100 * time.Millisecond) // Simulate 100ms ago start

	// Call finalizeStorageFromTracker BEFORE finishing the request
	handler.finalizeStorageFromTracker("test-req", supervisor.StatusSuccess, "", startTime)

	// Verify TTFB was saved to storage
	if savedTTFB == nil {
		t.Fatal("expected TTFB to be saved to storage, but it was nil")
	}

	if *savedTTFB <= 0 {
		t.Fatalf("expected positive TTFB value, got %d", *savedTTFB)
	}

	// Finish the request (removes from inFlight)
	tracker.Finish("test-req", supervisor.StatusSuccess, nil)

	// Verify request is gone from inFlight
	if info := tracker.GetRequestInfo("test-req"); info != nil {
		t.Error("expected request to be removed from inFlight after finish")
	}
}

// Mock storage implementation
type mockStore struct {
	updateFunc func(id string, upd storage.RequestUpdate)
}

func (m *mockStore) Insert(req *storage.Request) error {
	return nil
}

func (m *mockStore) Update(id string, upd storage.RequestUpdate) error {
	if m.updateFunc != nil {
		m.updateFunc(id, upd)
	}
	return nil
}

func (m *mockStore) GetByID(id string) (*storage.Request, error) {
	return nil, nil
}

func (m *mockStore) List(opts storage.ListOptions) ([]storage.Request, error) {
	return nil, nil
}

func (m *mockStore) Overview(window time.Duration) (*storage.Overview, error) {
	return nil, nil
}

func (m *mockStore) ModelStats(window time.Duration) ([]storage.ModelStat, error) {
	return nil, nil
}

func (m *mockStore) Series(opts storage.SeriesOptions) ([]storage.DataPoint, error) {
	return nil, nil
}

func (m *mockStore) InFlightCount() (int, error) {
	return 0, nil
}

func (m *mockStore) Close() error {
	return nil
}
