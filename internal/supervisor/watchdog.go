package supervisor

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Watchdog monitors in-flight requests and cancels them if they exceed timeout limits.
type Watchdog struct {
	tracker      *Tracker
	ttfbTimeout  time.Duration
	stallTimeout time.Duration
	hardTimeout  time.Duration
	logger       *slog.Logger
	restartHook  *RestartHook

	cancelFuncs map[string]context.CancelFunc
	mu          sync.RWMutex

	stopCh chan struct{}
}

// NewWatchdog creates a new watchdog instance.
func NewWatchdog(tracker *Tracker, ttfbTimeout, stallTimeout, hardTimeout time.Duration, logger *slog.Logger, restartHook *RestartHook) *Watchdog {
	return &Watchdog{
		tracker:      tracker,
		ttfbTimeout:  ttfbTimeout,
		stallTimeout: stallTimeout,
		hardTimeout:  hardTimeout,
		logger:       logger,
		restartHook:  restartHook,
		cancelFuncs:  make(map[string]context.CancelFunc),
		stopCh:       make(chan struct{}),
	}
}

// Start registers a request for monitoring with its cancel function.
func (w *Watchdog) Start(reqID string, cancelFunc context.CancelFunc) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.cancelFuncs[reqID] = cancelFunc
}

// Stop unregisters a request from monitoring (called when request finishes normally).
func (w *Watchdog) Stop(reqID string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	delete(w.cancelFuncs, reqID)
}

// Run starts the monitoring loop. This should be called in a separate goroutine.
// The loop runs until Shutdown() is called.
func (w *Watchdog) Run() {
	defer func() {
		// Fail-open: if watchdog panics, don't crash the whole proxy
		if r := recover(); r != nil {
			w.logger.Error("watchdog panic recovered", "panic", r)
		}
	}()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			w.logger.Debug("watchdog stopping")
			return
		case <-ticker.C:
			w.checkTimeouts()
		}
	}
}

// checkTimeouts examines all in-flight requests and cancels those that have timed out.
func (w *Watchdog) checkTimeouts() {
	// Get a snapshot of all in-flight requests
	snapshot := w.tracker.Snapshot()
	now := time.Now()

	// Check each in-flight request
	for reqID, req := range snapshot.InFlight {
		var timeoutType RequestStatus
		var shouldCancel bool

		// Check TTFB timeout: no bytes received at all
		if req.FirstByteTime == nil {
			if now.Sub(req.StartTime) > w.ttfbTimeout {
				timeoutType = StatusTimeoutTTFB
				shouldCancel = true
			}
		} else {
			// Check stall timeout: bytes started but no activity
			if now.Sub(req.LastActivityTime) > w.stallTimeout {
				timeoutType = StatusTimeoutStall
				shouldCancel = true
			}
		}

		// Check hard timeout: total wall-clock time
		if now.Sub(req.StartTime) > w.hardTimeout {
			timeoutType = StatusTimeoutHard
			shouldCancel = true
		}

		if shouldCancel {
			w.cancelRequest(reqID, timeoutType)
		}
	}
}

// cancelRequest cancels a request and records the timeout reason.
func (w *Watchdog) cancelRequest(reqID string, timeoutType RequestStatus) {
	// Get the cancel function
	w.mu.Lock()
	cancelFunc, exists := w.cancelFuncs[reqID]
	if !exists {
		w.mu.Unlock()
		return
	}
	// Remove from map to prevent double cancellation
	delete(w.cancelFuncs, reqID)
	w.mu.Unlock()

	// Cancel the request context
	cancelFunc()

	// Record the timeout in tracker
	w.tracker.Finish(reqID, timeoutType, nil)

	w.logger.Warn("request timed out", "req_id", reqID, "timeout_type", timeoutType)

	// Notify restart hook about the timeout
	if w.restartHook != nil {
		w.restartHook.RecordTimeout()
	}
}

// RecordSuccess notifies the restart hook of a successful request completion.
// This should be called by the handler when a request completes successfully.
func (w *Watchdog) RecordSuccess() {
	if w.restartHook != nil {
		w.restartHook.RecordSuccess()
	}
}

// Shutdown stops the monitoring loop gracefully.
func (w *Watchdog) Shutdown() {
	close(w.stopCh)
}