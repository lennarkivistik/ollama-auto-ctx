package supervisor

import (
	"testing"
	"time"
)

func TestMetrics_RecordRequest(t *testing.T) {
	metrics := NewMetrics()

	// Record a successful request
	metrics.RecordRequest("chat", "llama2", StatusSuccess, 1*time.Second, 1024, 256)

	// Record a failed request
	metrics.RecordRequest("generate", "codellama", StatusUpstreamError, 500*time.Millisecond, 0, 0)

	// Metrics are singletons, so we can't easily verify the values without Prometheus test helpers
	// But we can at least verify it doesn't panic
}

func TestMetrics_RecordTimeout(t *testing.T) {
	metrics := NewMetrics()

	metrics.RecordTimeout(StatusTimeoutTTFB)
	metrics.RecordTimeout(StatusTimeoutStall)
	metrics.RecordTimeout(StatusTimeoutHard)

	// Verify no panic
}

func TestMetrics_RecordLoopDetected(t *testing.T) {
	metrics := NewMetrics()

	metrics.RecordLoopDetected()
	metrics.RecordLoopDetected()

	// Verify no panic
}

func TestMetrics_RecordOutputLimitExceeded(t *testing.T) {
	metrics := NewMetrics()

	metrics.RecordOutputLimitExceeded("cancel")
	metrics.RecordOutputLimitExceeded("warn")

	// Verify no panic
}

func TestMetrics_UpdateInFlight(t *testing.T) {
	metrics := NewMetrics()

	metrics.UpdateInFlight(0)
	metrics.UpdateInFlight(5)
	metrics.UpdateInFlight(10)

	// Verify no panic
}

func TestMetrics_UpdateUpstreamHealth(t *testing.T) {
	metrics := NewMetrics()

	metrics.UpdateUpstreamHealth(true)
	metrics.UpdateUpstreamHealth(false)
	metrics.UpdateUpstreamHealth(true)

	// Verify no panic
}
