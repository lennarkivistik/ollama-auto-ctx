package supervisor

import (
	"encoding/json"
	"sync"
	"time"

	"ollama-auto-ctx/internal/calibration"
)

// EventType represents the type of lifecycle event.
type EventType string

const (
	EventRequestStart         EventType = "request_start"
	EventFirstByte            EventType = "first_byte"
	EventProgress             EventType = "progress"
	EventDone                 EventType = "done"
	EventCanceled             EventType = "canceled"
	EventTimeoutTTFB          EventType = "timeout_ttfb"
	EventTimeoutStall         EventType = "timeout_stall"
	EventTimeoutHard          EventType = "timeout_hard"
	EventUpstreamError        EventType = "upstream_error"
	EventLoopDetected         EventType = "loop_detected"
	EventOutputLimitExceeded  EventType = "output_limit_exceeded"
)

// Event represents a lifecycle event for a request.
type Event struct {
	Type                 EventType     `json:"type"`
	RequestID            string        `json:"request_id"`
	Timestamp            time.Time     `json:"timestamp"`
	Endpoint             string        `json:"endpoint,omitempty"`
	Model                string        `json:"model,omitempty"`
	BytesOut             int64         `json:"bytes_out"`
	EstimatedOutputTokens int64        `json:"estimated_output_tokens"`
	TTFBMs               int64         `json:"ttfb_ms"` // time to first byte in milliseconds, 0 if not yet received
	LastActivityAgeMs    int64         `json:"last_activity_age_ms"` // milliseconds since last activity
	Status               RequestStatus `json:"status,omitempty"`
	Error                string        `json:"error,omitempty"`
}

// EventBus manages event publishing and subscription for SSE consumers.
type EventBus struct {
	events     chan Event
	subscribers map[chan Event]struct{}
	mu         sync.RWMutex
	shutdown   chan struct{}
	once       sync.Once
}

// NewEventBus creates a new event bus with the specified buffer size.
func NewEventBus(bufferSize int) *EventBus {
	eb := &EventBus{
		events:      make(chan Event, bufferSize),
		subscribers: make(map[chan Event]struct{}),
		shutdown:    make(chan struct{}),
	}

	// Start forwarding goroutine
	go eb.forward()

	return eb
}

// forward forwards events from the main channel to all subscribers.
func (eb *EventBus) forward() {
	for {
		select {
		case event, ok := <-eb.events:
			if !ok {
				// Channel closed, shutdown
				return
			}
			eb.mu.RLock()
			subs := make([]chan Event, 0, len(eb.subscribers))
			for ch := range eb.subscribers {
				subs = append(subs, ch)
			}
			eb.mu.RUnlock()

			// Send to all subscribers (non-blocking)
			for _, ch := range subs {
				select {
				case ch <- event:
				default:
					// Subscriber channel is full, skip (fail-open)
				}
			}
		case <-eb.shutdown:
			// Shutdown signal received
			return
		}
	}
}

// Publish publishes an event. This is non-blocking and will drop events if the buffer is full.
func (eb *EventBus) Publish(event Event) {
	select {
	case eb.events <- event:
		// Event published successfully
	default:
		// Buffer full, drop event (fail-open)
	}
}

// Subscribe creates a new subscription channel for SSE consumers.
func (eb *EventBus) Subscribe() chan Event {
	ch := make(chan Event, 10) // Small buffer for subscriber
	eb.mu.Lock()
	eb.subscribers[ch] = struct{}{}
	eb.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscription channel and closes it.
func (eb *EventBus) Unsubscribe(ch chan Event) {
	eb.mu.Lock()
	if _, exists := eb.subscribers[ch]; exists {
		delete(eb.subscribers, ch)
		close(ch) // Signal to readers that no more events are coming
	}
	eb.mu.Unlock()
}

// Shutdown gracefully shuts down the event bus, closing the events channel and stopping the forward goroutine.
func (eb *EventBus) Shutdown() {
	eb.once.Do(func() {
		close(eb.shutdown)
		close(eb.events)
		
		// Close all subscriber channels
		eb.mu.Lock()
		for ch := range eb.subscribers {
			close(ch)
		}
		eb.subscribers = make(map[chan Event]struct{})
		eb.mu.Unlock()
	})
}

// EstimateOutputTokens estimates output tokens from bytes using calibration store.
func EstimateOutputTokens(bytes int64, model string, calibStore *calibration.Store, defaultTokensPerByte float64) int64 {
	if calibStore == nil {
		return int64(float64(bytes) * defaultTokensPerByte)
	}

	params := calibStore.Get(model)
	tokensPerByte := params.TokensPerByte
	if tokensPerByte <= 0 {
		tokensPerByte = defaultTokensPerByte
	}

	return int64(float64(bytes) * tokensPerByte)
}

// FormatSSEEvent formats an event as Server-Sent Events format.
func FormatSSEEvent(event Event) (string, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	return "data: " + string(data) + "\n\n", nil
}