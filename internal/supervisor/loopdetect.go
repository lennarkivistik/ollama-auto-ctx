package supervisor

import (
	"context"
	"sync"
)

// LoopDetector detects repetitive output patterns in streaming responses.
// It uses a rolling n-gram detection approach to identify when a model is
// producing degenerate repeating output.
//
// The detector is fail-open: if parsing fails or detection logic errors,
// it will not trigger cancellation.
type LoopDetector struct {
	windowSize      int // size of rolling window in bytes
	ngramSize       int // size of n-grams to detect
	repeatThreshold int // number of repeats needed to trigger
	minOutputBytes  int // minimum output before detection activates

	mu         sync.Mutex
	buffer     []byte           // rolling window buffer
	ngramCount map[string]int   // count of each n-gram
	totalBytes int64            // total bytes seen
	triggered  bool             // whether loop was already detected
	cancelFunc context.CancelFunc
	requestID  string
	tracker    *Tracker
}

// LoopDetectorConfig holds configuration for loop detection.
type LoopDetectorConfig struct {
	WindowBytes     int // SUPERVISOR_LOOP_WINDOW_BYTES (default 4096)
	NgramBytes      int // SUPERVISOR_LOOP_NGRAM_BYTES (default 64)
	RepeatThreshold int // SUPERVISOR_LOOP_REPEAT_THRESHOLD (default 3)
	MinOutputBytes  int // SUPERVISOR_LOOP_MIN_OUTPUT_BYTES (default 1024)
}

// NewLoopDetector creates a new loop detector for a request.
func NewLoopDetector(cfg LoopDetectorConfig, requestID string, cancelFunc context.CancelFunc, tracker *Tracker) *LoopDetector {
	// Apply sensible defaults and minimums
	windowSize := cfg.WindowBytes
	if windowSize < 256 {
		windowSize = 256
	}
	ngramSize := cfg.NgramBytes
	if ngramSize < 8 {
		ngramSize = 8
	}
	if ngramSize > windowSize/2 {
		ngramSize = windowSize / 2
	}
	repeatThreshold := cfg.RepeatThreshold
	if repeatThreshold < 2 {
		repeatThreshold = 2
	}
	minOutput := cfg.MinOutputBytes
	if minOutput < 256 {
		minOutput = 256
	}

	return &LoopDetector{
		windowSize:      windowSize,
		ngramSize:       ngramSize,
		repeatThreshold: repeatThreshold,
		minOutputBytes:  minOutput,
		buffer:          make([]byte, 0, windowSize),
		ngramCount:      make(map[string]int),
		cancelFunc:      cancelFunc,
		requestID:       requestID,
		tracker:         tracker,
	}
}

// Feed adds bytes to the detector and checks for loops.
// Returns true if a loop was detected and cancellation triggered.
// This method is safe to call concurrently.
func (ld *LoopDetector) Feed(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	ld.mu.Lock()
	defer ld.mu.Unlock()

	// Already triggered, don't process more
	if ld.triggered {
		return true
	}

	ld.totalBytes += int64(len(data))

	// Don't check until we have minimum output
	if ld.totalBytes < int64(ld.minOutputBytes) {
		// Still accumulate in buffer for when we cross threshold
		ld.addToBuffer(data)
		return false
	}

	// Add to rolling buffer
	ld.addToBuffer(data)

	// Check for repeated n-grams
	if ld.checkForLoop() {
		ld.triggered = true
		ld.triggerCancellation()
		return true
	}

	return false
}

// addToBuffer adds data to the rolling buffer, maintaining window size.
func (ld *LoopDetector) addToBuffer(data []byte) {
	// Append data
	ld.buffer = append(ld.buffer, data...)

	// Trim to window size if needed
	if len(ld.buffer) > ld.windowSize {
		// Remove old n-grams that will be trimmed
		excess := len(ld.buffer) - ld.windowSize
		for i := 0; i <= excess-ld.ngramSize && i+ld.ngramSize <= len(ld.buffer); i++ {
			ngram := string(ld.buffer[i : i+ld.ngramSize])
			if count, exists := ld.ngramCount[ngram]; exists {
				if count <= 1 {
					delete(ld.ngramCount, ngram)
				} else {
					ld.ngramCount[ngram] = count - 1
				}
			}
		}
		// Trim buffer
		ld.buffer = ld.buffer[excess:]
	}

	// Add new n-grams from the added data
	// We need to add n-grams that span the boundary between old and new data
	startPos := len(ld.buffer) - len(data) - ld.ngramSize + 1
	if startPos < 0 {
		startPos = 0
	}
	for i := startPos; i+ld.ngramSize <= len(ld.buffer); i++ {
		ngram := string(ld.buffer[i : i+ld.ngramSize])
		ld.ngramCount[ngram]++
	}
}

// checkForLoop checks if any n-gram exceeds the repeat threshold.
func (ld *LoopDetector) checkForLoop() bool {
	for _, count := range ld.ngramCount {
		if count >= ld.repeatThreshold {
			return true
		}
	}
	return false
}

// triggerCancellation cancels the request and records the loop detection.
func (ld *LoopDetector) triggerCancellation() {
	if ld.cancelFunc != nil {
		ld.cancelFunc()
	}
	if ld.tracker != nil {
		ld.tracker.Finish(ld.requestID, StatusLoopDetected, nil)
	}
}

// Triggered returns whether a loop was detected.
func (ld *LoopDetector) Triggered() bool {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	return ld.triggered
}

// Reset clears the detector state for reuse.
func (ld *LoopDetector) Reset() {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	ld.buffer = ld.buffer[:0]
	ld.ngramCount = make(map[string]int)
	ld.totalBytes = 0
	ld.triggered = false
}
