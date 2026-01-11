package supervisor

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

func TestWatchdog_TTFBTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tracker := NewTracker(10, nil, nil, 0.25, 250*time.Millisecond, nil)
	watchdog := NewWatchdog(tracker, 100*time.Millisecond, 1*time.Second, 10*time.Second, logger, nil)

	// Start watchdog
	go watchdog.Run()
	defer watchdog.Shutdown()

	// Create a cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start monitoring a request
	reqID := "test-ttfb"
	watchdog.Start(reqID, cancel)
	tracker.Start(reqID, "/api/chat", "model", false)

	// Wait for TTFB timeout (need to wait longer than 1 check interval)
	time.Sleep(1200 * time.Millisecond) // Wait > 1 second for watchdog check

	// Check that request was canceled and marked as timed out
	if ctx.Err() != context.Canceled {
		t.Error("expected context to be canceled due to TTFB timeout")
	}

	// Check tracker status - request should be in recent after timeout
	snapshot := tracker.Snapshot()
	if len(snapshot.InFlight) > 0 {
		t.Error("expected no requests in flight after timeout")
	}
	if len(snapshot.Recent) == 0 {
		t.Error("expected request to be in recent buffer")
	}

	recent := snapshot.Recent[len(snapshot.Recent)-1]
	if recent.Status != StatusTimeoutTTFB {
		t.Errorf("expected status %s, got %s", StatusTimeoutTTFB, recent.Status)
	}
}

func TestWatchdog_StallTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tracker := NewTracker(10, nil, nil, 0.25, 250*time.Millisecond, nil)
	watchdog := NewWatchdog(tracker, 10*time.Second, 100*time.Millisecond, 10*time.Second, logger, nil)

	// Start watchdog
	go watchdog.Run()
	defer watchdog.Shutdown()

	// Create a cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start monitoring a request
	reqID := "test-stall"
	watchdog.Start(reqID, cancel)
	tracker.Start(reqID, "/api/chat", "model", false)

	// Mark first byte received (but don't update activity time)
	tracker.MarkFirstByte(reqID)

	// Wait for stall timeout (need to wait longer than 1 check interval)
	time.Sleep(1200 * time.Millisecond) // Wait > 1 second for watchdog check

	// Check that request was canceled and marked as timed out
	if ctx.Err() != context.Canceled {
		t.Error("expected context to be canceled due to stall timeout")
	}

	// Check tracker status
	snapshot := tracker.Snapshot()
	if len(snapshot.InFlight) > 0 {
		t.Error("expected no requests in flight after timeout")
	}
	if len(snapshot.Recent) == 0 {
		t.Error("expected request to be in recent buffer")
	}

	recent := snapshot.Recent[len(snapshot.Recent)-1]
	if recent.Status != StatusTimeoutStall {
		t.Errorf("expected status %s, got %s", StatusTimeoutStall, recent.Status)
	}
}

func TestWatchdog_HardTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tracker := NewTracker(10, nil, nil, 0.25, 250*time.Millisecond, nil)
	watchdog := NewWatchdog(tracker, 10*time.Second, 10*time.Second, 100*time.Millisecond, logger, nil)

	// Start watchdog
	go watchdog.Run()
	defer watchdog.Shutdown()

	// Create a cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start monitoring a request
	reqID := "test-hard"
	watchdog.Start(reqID, cancel)
	tracker.Start(reqID, "/api/chat", "model", false)

	// Mark first byte to avoid TTFB timeout
	tracker.MarkFirstByte(reqID)

	// Keep activity updated to avoid stall timeout, but let hard timeout trigger
	// Wait longer than watchdog check interval (1 second) for hard timeout detection
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(1200 * time.Millisecond) // Wait longer for watchdog check
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-timeout:
				done <- true
				return
			case <-ticker.C:
				tracker.MarkProgress(reqID, 1) // Keep activity updated
			}
		}
	}()

	<-done

	// Check that request was canceled and marked as timed out
	if ctx.Err() != context.Canceled {
		t.Errorf("expected context to be canceled due to hard timeout, got: %v", ctx.Err())
		return
	}

	// Check tracker status
	snapshot := tracker.Snapshot()
	if len(snapshot.Recent) == 0 {
		t.Error("expected request to be in recent buffer")
		return
	}

	recent := snapshot.Recent[len(snapshot.Recent)-1]
	if recent.Status != StatusTimeoutHard {
		t.Errorf("expected status %s, got %s", StatusTimeoutHard, recent.Status)
	}
}

func TestWatchdog_NoTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tracker := NewTracker(10, nil, nil, 0.25, 250*time.Millisecond, nil)
	watchdog := NewWatchdog(tracker, 200*time.Millisecond, 200*time.Millisecond, 500*time.Millisecond, logger, nil)

	// Start watchdog
	go watchdog.Run()
	defer watchdog.Shutdown()

	// Create a cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start monitoring a request
	reqID := "test-no-timeout"
	watchdog.Start(reqID, cancel)
	tracker.Start(reqID, "/api/chat", "model", false)

	// Keep activity updated and finish before timeouts
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for i := 0; i < 5; i++ {
			<-ticker.C
			tracker.MarkProgress(reqID, 1)
		}

		// Finish the request normally
		watchdog.Stop(reqID)
		tracker.Finish(reqID, StatusSuccess, nil)
	}()

	// Wait a bit longer than stall timeout (need to wait longer than 1 check interval)
	time.Sleep(1200 * time.Millisecond)

	// Check that context was not canceled
	if ctx.Err() == context.Canceled {
		t.Error("expected context to not be canceled")
	}
}

func TestWatchdog_ConcurrentRequests(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tracker := NewTracker(10, nil, nil, 0.25, 250*time.Millisecond, nil)
	watchdog := NewWatchdog(tracker, 100*time.Millisecond, 1*time.Second, 10*time.Second, logger, nil)

	// Start watchdog
	go watchdog.Run()
	defer watchdog.Shutdown()

	const numRequests = 5
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func(reqNum int) {
			defer wg.Done()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			reqID := "concurrent-" + string(rune('0'+reqNum))
			watchdog.Start(reqID, cancel)
			tracker.Start(reqID, "/api/chat", "model", false)

			if reqNum%2 == 0 {
				// Even numbered requests: let them timeout
				// Wait longer than watchdog check interval (1 second)
				time.Sleep(1200 * time.Millisecond)
				if ctx.Err() != context.Canceled {
					t.Errorf("request %d should have been canceled", reqNum)
				}
			} else {
				// Odd numbered requests: finish normally (before TTFB timeout)
				time.Sleep(50 * time.Millisecond)
				watchdog.Stop(reqID)
				tracker.Finish(reqID, StatusSuccess, nil)
				if ctx.Err() == context.Canceled {
					t.Errorf("request %d should not have been canceled", reqNum)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestWatchdog_Shutdown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	tracker := NewTracker(10, nil, nil, 0.25, 250*time.Millisecond, nil)
	watchdog := NewWatchdog(tracker, 1*time.Second, 1*time.Second, 1*time.Second, logger, nil)

	// Start watchdog
	done := make(chan bool)
	go func() {
		watchdog.Run()
		done <- true
	}()

	// Shutdown immediately
	watchdog.Shutdown()

	select {
	case <-done:
		// Expected - watchdog shut down
	case <-time.After(100 * time.Millisecond):
		t.Error("watchdog did not shut down within timeout")
	}
}