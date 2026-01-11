package supervisor

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEventBus_PublishSubscribe(t *testing.T) {
	bus := NewEventBus(10)

	sub := bus.Subscribe()
	defer bus.Unsubscribe(sub)

	event := Event{
		Type:      EventRequestStart,
		RequestID: "test-1",
		Timestamp: time.Now(),
	}

	bus.Publish(event)

	select {
	case received := <-sub:
		if received.RequestID != "test-1" {
			t.Errorf("expected request ID test-1, got %s", received.RequestID)
		}
		if received.Type != EventRequestStart {
			t.Errorf("expected type %s, got %s", EventRequestStart, received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive event within timeout")
	}
}

func TestEventBus_BoundedBuffer(t *testing.T) {
	bus := NewEventBus(2) // Small buffer

	sub := bus.Subscribe()
	defer bus.Unsubscribe(sub)

	// Publish more events than buffer size
	for i := 0; i < 5; i++ {
		event := Event{
			Type:      EventProgress,
			RequestID:  "test",
			Timestamp: time.Now(),
		}
		bus.Publish(event)
	}

	// Should receive at least some events (non-blocking publish may drop some)
	received := 0
	timeout := time.After(100 * time.Millisecond)
	for {
		select {
		case <-sub:
			received++
		case <-timeout:
			goto done
		}
	}
done:
	if received == 0 {
		t.Error("expected to receive at least one event")
	}
}

func TestEventBus_NonBlockingPublish(t *testing.T) {
	bus := NewEventBus(1) // Very small buffer

	// Publish many events rapidly - should not block
	start := time.Now()
	for i := 0; i < 100; i++ {
		event := Event{
			Type:      EventProgress,
			RequestID: "test",
			Timestamp: time.Now(),
		}
		bus.Publish(event) // Should not block even if buffer is full
	}
	duration := time.Since(start)

	// Should complete very quickly (non-blocking)
	if duration > 10*time.Millisecond {
		t.Errorf("publish took too long: %v (should be non-blocking)", duration)
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewEventBus(10)

	sub1 := bus.Subscribe()
	defer bus.Unsubscribe(sub1)
	sub2 := bus.Subscribe()
	defer bus.Unsubscribe(sub2)

	event := Event{
		Type:      EventRequestStart,
		RequestID: "test-1",
		Timestamp: time.Now(),
	}

	bus.Publish(event)

	// Both subscribers should receive the event
	received1 := false
	received2 := false

	timeout := time.After(100 * time.Millisecond)
	for !received1 || !received2 {
		select {
		case <-sub1:
			received1 = true
		case <-sub2:
			received2 = true
		case <-timeout:
			if !received1 {
				t.Error("subscriber 1 did not receive event")
			}
			if !received2 {
				t.Error("subscriber 2 did not receive event")
			}
			return
		}
	}
}

func TestEstimateOutputTokens(t *testing.T) {
	// Test with nil calibration store (uses default)
	tokens := EstimateOutputTokens(100, "model1", nil, 0.25)
	if tokens != 25 {
		t.Errorf("expected 25 tokens, got %d", tokens)
	}

	// Test with zero bytes
	tokens = EstimateOutputTokens(0, "model1", nil, 0.25)
	if tokens != 0 {
		t.Errorf("expected 0 tokens, got %d", tokens)
	}

	// Test with large number
	tokens = EstimateOutputTokens(10000, "model1", nil, 0.25)
	if tokens != 2500 {
		t.Errorf("expected 2500 tokens, got %d", tokens)
	}
}

func TestFormatSSEEvent(t *testing.T) {
	event := Event{
		Type:      EventRequestStart,
		RequestID: "test-1",
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Endpoint:  "/api/chat",
	}

	sse, err := FormatSSEEvent(event)
	if err != nil {
		t.Fatalf("failed to format SSE event: %v", err)
	}

	if !json.Valid([]byte(sse[6 : len(sse)-2])) { // Skip "data: " and "\n\n"
		t.Error("SSE data is not valid JSON")
	}

	if !contains(sse, "data: ") {
		t.Error("SSE format should start with 'data: '")
	}

	if !contains(sse, "\n\n") {
		t.Error("SSE format should end with '\\n\\n'")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || strings.Contains(s, substr))
}