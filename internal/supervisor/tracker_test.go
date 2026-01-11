package supervisor

import (
	"sync"
	"testing"
	"time"
)

func TestTracker_BasicOperations(t *testing.T) {
	tracker := NewTracker(10, nil, nil, 0.25, 250*time.Millisecond, nil)

	// Start a request
	tracker.Start("req1", "/api/chat", "llama2", true)

	// Check it's in flight
	snapshot := tracker.Snapshot()
	if len(snapshot.InFlight) != 1 {
		t.Errorf("expected 1 in-flight request, got %d", len(snapshot.InFlight))
	}

	req, exists := snapshot.InFlight["req1"]
	if !exists {
		t.Fatal("request req1 not found in in-flight")
	}

	if req.ID != "req1" {
		t.Errorf("expected ID req1, got %s", req.ID)
	}
	if req.Endpoint != "/api/chat" {
		t.Errorf("expected endpoint /api/chat, got %s", req.Endpoint)
	}
	if req.Model != "llama2" {
		t.Errorf("expected model llama2, got %s", req.Model)
	}
	if !req.ClientRequestedStream {
		t.Error("expected stream=true")
	}

	// Mark first byte
	tracker.MarkFirstByte("req1")
	snapshot = tracker.Snapshot()
	req = snapshot.InFlight["req1"]
	if req.FirstByteTime == nil {
		t.Error("expected FirstByteTime to be set")
	}

	// Mark progress
	tracker.MarkProgress("req1", 1024)
	snapshot = tracker.Snapshot()
	req = snapshot.InFlight["req1"]
	if req.BytesForwarded != 1024 {
		t.Errorf("expected 1024 bytes, got %d", req.BytesForwarded)
	}

	// Finish request
	tracker.Finish("req1", StatusSuccess, nil)

	// Check it's moved to recent
	snapshot = tracker.Snapshot()
	if len(snapshot.InFlight) != 0 {
		t.Errorf("expected 0 in-flight requests, got %d", len(snapshot.InFlight))
	}
	if len(snapshot.Recent) != 1 {
		t.Errorf("expected 1 recent request, got %d", len(snapshot.Recent))
	}

	recent := snapshot.Recent[0]
	if recent.Status != StatusSuccess {
		t.Errorf("expected status success, got %s", recent.Status)
	}
}

func TestTracker_RingBuffer(t *testing.T) {
	maxRecent := 3
	tracker := NewTracker(maxRecent, nil, nil, 0.25, 250*time.Millisecond, nil)

	// Add requests beyond the buffer size
	for i := 1; i <= maxRecent+2; i++ {
		reqID := "req" + string(rune('0'+i))
		tracker.Start(reqID, "/api/chat", "model", false)
		tracker.Finish(reqID, StatusSuccess, nil)
	}

	snapshot := tracker.Snapshot()
	if len(snapshot.Recent) != maxRecent {
		t.Errorf("expected %d recent requests, got %d", maxRecent, len(snapshot.Recent))
	}

	// The oldest requests should be evicted
	expectedIDs := []string{"req3", "req4", "req5"} // req1 and req2 should be evicted
	actualIDs := make([]string, 0, maxRecent)
	for _, req := range snapshot.Recent {
		actualIDs = append(actualIDs, req.ID)
	}

	for i, expected := range expectedIDs {
		if actualIDs[i] != expected {
			t.Errorf("expected recent[%d] to be %s, got %s", i, expected, actualIDs[i])
		}
	}
}

func TestTracker_ConcurrentOperations(t *testing.T) {
	tracker := NewTracker(100, nil, nil, 0.25, 250*time.Millisecond, nil)
	const numGoroutines = 10
	const requestsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()

			for r := 0; r < requestsPerGoroutine; r++ {
				reqID := "req" + string(rune('0'+goroutineID)) + "-" + string(rune('0'+r))

				// Start request
				tracker.Start(reqID, "/api/generate", "model"+string(rune('0'+goroutineID)), false)

				// Simulate some progress
				tracker.MarkProgress(reqID, 512)
				time.Sleep(time.Millisecond) // Small delay to simulate work

				// Mark first byte
				tracker.MarkFirstByte(reqID)

				// More progress
				tracker.MarkProgress(reqID, 512)

				// Finish
				tracker.Finish(reqID, StatusSuccess, nil)
			}
		}(g)
	}

	wg.Wait()

	// All requests should be finished
	snapshot := tracker.Snapshot()
	if len(snapshot.InFlight) != 0 {
		t.Errorf("expected 0 in-flight requests after concurrent operations, got %d", len(snapshot.InFlight))
	}

	expectedRecent := numGoroutines * requestsPerGoroutine
	if len(snapshot.Recent) != expectedRecent {
		t.Errorf("expected %d recent requests, got %d", expectedRecent, len(snapshot.Recent))
	}

	// Verify all recent requests have correct status
	for _, req := range snapshot.Recent {
		if req.Status != StatusSuccess {
			t.Errorf("request %s has status %s, expected success", req.ID, req.Status)
		}
		if req.BytesForwarded != 1024 {
			t.Errorf("request %s has %d bytes, expected 1024", req.ID, req.BytesForwarded)
		}
		if req.FirstByteTime == nil {
			t.Errorf("request %s missing FirstByteTime", req.ID)
		}
	}
}

func TestTracker_UpdateModel(t *testing.T) {
	tracker := NewTracker(10, nil, nil, 0.25, 250*time.Millisecond, nil)

	tracker.Start("req1", "/api/chat", "", false)

	// Update model
	tracker.UpdateModel("req1", "llama2:latest")

	snapshot := tracker.Snapshot()
	req := snapshot.InFlight["req1"]
	if req.Model != "llama2:latest" {
		t.Errorf("expected model llama2:latest, got %s", req.Model)
	}
}

func TestTracker_NonExistentRequest(t *testing.T) {
	tracker := NewTracker(10, nil, nil, 0.25, 250*time.Millisecond, nil)

	// Operations on non-existent requests should not panic
	tracker.MarkFirstByte("nonexistent")
	tracker.MarkProgress("nonexistent", 100)
	tracker.UpdateModel("nonexistent", "model")
	tracker.Finish("nonexistent", StatusSuccess, nil)

	// Should not affect any real requests
	snapshot := tracker.Snapshot()
	if len(snapshot.InFlight) != 0 {
		t.Errorf("expected 0 in-flight requests, got %d", len(snapshot.InFlight))
	}
}