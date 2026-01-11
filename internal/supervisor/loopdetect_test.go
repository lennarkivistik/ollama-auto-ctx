package supervisor

import (
	"context"
	"strings"
	"testing"
)

func TestLoopDetector_NonRepeatingText(t *testing.T) {
	cfg := LoopDetectorConfig{
		WindowBytes:     512,
		NgramBytes:      16,
		RepeatThreshold: 3,
		MinOutputBytes:  100,
	}

	detector := NewLoopDetector(cfg, "test-req", nil, nil)

	// Feed non-repeating text
	texts := []string{
		"Hello, this is a test. ",
		"The quick brown fox jumps over the lazy dog. ",
		"Pack my box with five dozen liquor jugs. ",
		"How vexingly quick daft zebras jump! ",
		"Sphinx of black quartz, judge my vow. ",
	}

	for _, text := range texts {
		if detector.Feed([]byte(text)) {
			t.Errorf("loop detected incorrectly for non-repeating text")
		}
	}

	if detector.Triggered() {
		t.Errorf("detector should not have triggered for non-repeating text")
	}
}

func TestLoopDetector_RepeatingPattern(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := LoopDetectorConfig{
		WindowBytes:     512,
		NgramBytes:      16,
		RepeatThreshold: 3,
		MinOutputBytes:  100,
	}

	var cancelled bool
	cancelFunc := func() {
		cancelled = true
		cancel()
	}

	detector := NewLoopDetector(cfg, "test-req", cancelFunc, nil)

	// Generate repeating pattern that exceeds minimum output
	pattern := strings.Repeat("This is a repeating pattern. ", 50)

	detector.Feed([]byte(pattern))

	if !detector.Triggered() {
		t.Errorf("detector should have triggered for repeating pattern")
	}

	if !cancelled {
		t.Errorf("cancel function should have been called")
	}
}

func TestLoopDetector_BelowMinimumOutput(t *testing.T) {
	cfg := LoopDetectorConfig{
		WindowBytes:     512,
		NgramBytes:      16,
		RepeatThreshold: 3,
		MinOutputBytes:  1000, // High minimum
	}

	var cancelled bool
	cancelFunc := func() {
		cancelled = true
	}

	detector := NewLoopDetector(cfg, "test-req", cancelFunc, nil)

	// Feed repeating pattern but below minimum output threshold
	pattern := strings.Repeat("repeat ", 20) // Only ~140 bytes

	detector.Feed([]byte(pattern))

	if detector.Triggered() {
		t.Errorf("detector should not trigger below minimum output threshold")
	}

	if cancelled {
		t.Errorf("cancel function should not have been called")
	}
}

func TestLoopDetector_ThresholdBoundary(t *testing.T) {
	cfg := LoopDetectorConfig{
		WindowBytes:     512,
		NgramBytes:      8,
		RepeatThreshold: 3,
		MinOutputBytes:  256, // Minimum enforced by NewLoopDetector
	}

	detector := NewLoopDetector(cfg, "test-req", nil, nil)

	// Feed enough data to exceed minOutput (256 bytes)
	// The pattern "12345678" appears many times in sequence
	data := strings.Repeat("12345678", 40) // 320 bytes, 40 occurrences of the pattern

	detector.Feed([]byte(data))
	if !detector.Triggered() {
		t.Errorf("should trigger with repeated n-gram (repeated 40 times)")
	}
}

func TestLoopDetector_Reset(t *testing.T) {
	cfg := LoopDetectorConfig{
		WindowBytes:     512,
		NgramBytes:      16,
		RepeatThreshold: 3,
		MinOutputBytes:  100,
	}

	detector := NewLoopDetector(cfg, "test-req", nil, nil)

	// Feed repeating pattern
	pattern := strings.Repeat("This repeats. ", 50)
	detector.Feed([]byte(pattern))

	if !detector.Triggered() {
		t.Errorf("should have triggered")
	}

	// Reset
	detector.Reset()

	if detector.Triggered() {
		t.Errorf("should not be triggered after reset")
	}
}

func TestLoopDetector_ConcurrentAccess(t *testing.T) {
	cfg := LoopDetectorConfig{
		WindowBytes:     512,
		NgramBytes:      16,
		RepeatThreshold: 10, // High threshold to avoid triggering
		MinOutputBytes:  50,
	}

	detector := NewLoopDetector(cfg, "test-req", nil, nil)

	done := make(chan bool)

	// Concurrent feeds
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				detector.Feed([]byte("some text data "))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Just verify it didn't panic
	_ = detector.Triggered()
}
