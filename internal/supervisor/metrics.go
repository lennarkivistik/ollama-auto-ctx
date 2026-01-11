package supervisor

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics collects Prometheus metrics for the proxy.
type Metrics struct {
	requestsTotal          *prometheus.CounterVec
	requestDuration        *prometheus.HistogramVec
	bytesOut               *prometheus.CounterVec
	tokensEstimated        *prometheus.CounterVec
	inFlightRequests       prometheus.Gauge
	upstreamHealthy        prometheus.Gauge
	timeoutsTotal          *prometheus.CounterVec
	loopsDetected          prometheus.Counter
	outputLimitExceeded    *prometheus.CounterVec
}

var (
	metricsOnce sync.Once
	metricsInst *Metrics
)

// NewMetrics creates a new metrics collector.
func NewMetrics() *Metrics {
	metricsOnce.Do(func() {
		metricsInst = &Metrics{
			requestsTotal: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "ollama_proxy_requests_total",
					Help: "Total number of requests processed",
				},
				[]string{"endpoint", "model", "status"},
			),
			requestDuration: promauto.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "ollama_proxy_request_duration_seconds",
					Help:    "Request duration in seconds",
					Buckets: prometheus.DefBuckets, // 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
				},
				[]string{"endpoint", "model"},
			),
			bytesOut: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "ollama_proxy_bytes_out_total",
					Help: "Total bytes forwarded to clients",
				},
				[]string{"model"},
			),
			tokensEstimated: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "ollama_proxy_tokens_estimated_total",
					Help: "Total estimated output tokens",
				},
				[]string{"model"},
			),
			inFlightRequests: promauto.NewGauge(
				prometheus.GaugeOpts{
					Name: "ollama_proxy_requests_in_flight",
					Help: "Number of requests currently in flight",
				},
			),
			upstreamHealthy: promauto.NewGauge(
				prometheus.GaugeOpts{
					Name: "ollama_proxy_upstream_healthy",
					Help: "Upstream Ollama health status (1 = healthy, 0 = unhealthy)",
				},
			),
			timeoutsTotal: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "ollama_proxy_timeouts_total",
					Help: "Total number of timeouts by type",
				},
				[]string{"type"},
			),
			loopsDetected: promauto.NewCounter(
				prometheus.CounterOpts{
					Name: "ollama_proxy_loops_detected_total",
					Help: "Total number of loops detected",
				},
			),
			outputLimitExceeded: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "ollama_proxy_output_limit_exceeded_total",
					Help: "Total number of times output token limit was exceeded",
				},
				[]string{"action"},
			),
		}
	})
	return metricsInst
}

// RecordRequest records a completed request.
func (m *Metrics) RecordRequest(endpoint, model string, status RequestStatus, duration time.Duration, bytesOut, tokensEstimated int64) {
	if m == nil {
		return
	}

	// Normalize model name (empty -> "unknown")
	modelLabel := model
	if modelLabel == "" {
		modelLabel = "unknown"
	}

	// Normalize endpoint (empty -> "unknown")
	endpointLabel := endpoint
	if endpointLabel == "" {
		endpointLabel = "unknown"
	}

	// Normalize status
	statusLabel := string(status)
	if statusLabel == "" {
		statusLabel = "unknown"
	}

	m.requestsTotal.WithLabelValues(endpointLabel, modelLabel, statusLabel).Inc()
	m.requestDuration.WithLabelValues(endpointLabel, modelLabel).Observe(duration.Seconds())

	if bytesOut > 0 {
		m.bytesOut.WithLabelValues(modelLabel).Add(float64(bytesOut))
	}

	if tokensEstimated > 0 {
		m.tokensEstimated.WithLabelValues(modelLabel).Add(float64(tokensEstimated))
	}
}

// RecordTimeout records a timeout event.
func (m *Metrics) RecordTimeout(timeoutType RequestStatus) {
	if m == nil {
		return
	}

	var typeLabel string
	switch timeoutType {
	case StatusTimeoutTTFB:
		typeLabel = "ttfb"
	case StatusTimeoutStall:
		typeLabel = "stall"
	case StatusTimeoutHard:
		typeLabel = "hard"
	default:
		typeLabel = "unknown"
	}

	m.timeoutsTotal.WithLabelValues(typeLabel).Inc()
}

// RecordLoopDetected records a loop detection event.
func (m *Metrics) RecordLoopDetected() {
	if m == nil {
		return
	}
	m.loopsDetected.Inc()
}

// RecordOutputLimitExceeded records an output limit exceeded event.
func (m *Metrics) RecordOutputLimitExceeded(action string) {
	if m == nil {
		return
	}
	m.outputLimitExceeded.WithLabelValues(action).Inc()
}

// UpdateInFlight updates the in-flight requests gauge.
func (m *Metrics) UpdateInFlight(count int) {
	if m == nil {
		return
	}
	m.inFlightRequests.Set(float64(count))
}

// UpdateUpstreamHealth updates the upstream health gauge.
func (m *Metrics) UpdateUpstreamHealth(healthy bool) {
	if m == nil {
		return
	}
	if healthy {
		m.upstreamHealthy.Set(1)
	} else {
		m.upstreamHealthy.Set(0)
	}
}
