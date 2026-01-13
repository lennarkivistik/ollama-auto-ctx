package supervisor

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics collects Prometheus metrics for the proxy.
// Labels are kept to low-cardinality: model, status, reason.
type Metrics struct {
	// Counters
	requestsTotal   *prometheus.CounterVec // model, status, reason
	retriesTotal    *prometheus.CounterVec // model

	// Histograms
	requestDuration *prometheus.HistogramVec // model
	ttfbSeconds     *prometheus.HistogramVec // model

	// Gauges
	inFlightRequests prometheus.Gauge
	upstreamHealthy  prometheus.Gauge
}

var (
	metricsOnce sync.Once
	metricsInst *Metrics
)

// NewMetrics creates a new metrics collector.
// Uses low-cardinality labels as specified.
func NewMetrics() *Metrics {
	metricsOnce.Do(func() {
		metricsInst = &Metrics{
			requestsTotal: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "oac_requests_total",
					Help: "Total number of requests processed",
				},
				[]string{"model", "status", "reason"},
			),
			retriesTotal: promauto.NewCounterVec(
				prometheus.CounterOpts{
					Name: "oac_retries_total",
					Help: "Total number of retries",
				},
				[]string{"model"},
			),
			requestDuration: promauto.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "oac_request_duration_seconds",
					Help:    "Request duration in seconds",
					Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300},
				},
				[]string{"model"},
			),
			ttfbSeconds: promauto.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:    "oac_ttfb_seconds",
					Help:    "Time to first byte in seconds",
					Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
				},
				[]string{"model"},
			),
			inFlightRequests: promauto.NewGauge(
				prometheus.GaugeOpts{
					Name: "oac_requests_in_flight",
					Help: "Number of requests currently in flight",
				},
			),
			upstreamHealthy: promauto.NewGauge(
				prometheus.GaugeOpts{
					Name: "oac_upstream_healthy",
					Help: "Upstream Ollama health status (1 = healthy, 0 = unhealthy)",
				},
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

	modelLabel := model
	if modelLabel == "" {
		modelLabel = "unknown"
	}

	statusLabel := "success"
	reasonLabel := ""

	switch status {
	case StatusSuccess:
		statusLabel = "success"
	case StatusCanceled:
		statusLabel = "canceled"
	case StatusTimeoutTTFB:
		statusLabel = "error"
		reasonLabel = "timeout_ttfb"
	case StatusTimeoutStall:
		statusLabel = "error"
		reasonLabel = "timeout_stall"
	case StatusTimeoutHard:
		statusLabel = "error"
		reasonLabel = "timeout_hard"
	case StatusUpstreamError:
		statusLabel = "error"
		reasonLabel = "upstream_error"
	case StatusLoopDetected:
		statusLabel = "error"
		reasonLabel = "loop_detected"
	case StatusOutputLimitExceeded:
		statusLabel = "error"
		reasonLabel = "output_limit"
	default:
		statusLabel = string(status)
	}

	m.requestsTotal.WithLabelValues(modelLabel, statusLabel, reasonLabel).Inc()
	m.requestDuration.WithLabelValues(modelLabel).Observe(duration.Seconds())
}

// RecordTTFB records time to first byte.
func (m *Metrics) RecordTTFB(model string, ttfb time.Duration) {
	if m == nil {
		return
	}

	modelLabel := model
	if modelLabel == "" {
		modelLabel = "unknown"
	}

	m.ttfbSeconds.WithLabelValues(modelLabel).Observe(ttfb.Seconds())
}

// RecordRetry records a retry attempt.
func (m *Metrics) RecordRetry(model string) {
	if m == nil {
		return
	}

	modelLabel := model
	if modelLabel == "" {
		modelLabel = "unknown"
	}

	m.retriesTotal.WithLabelValues(modelLabel).Inc()
}

// RecordTimeout records a timeout event (deprecated, use RecordRequest).
func (m *Metrics) RecordTimeout(timeoutType RequestStatus) {
	// Now handled by RecordRequest with reason label
}

// RecordLoopDetected records a loop detection event (deprecated, use RecordRequest).
func (m *Metrics) RecordLoopDetected() {
	// Now handled by RecordRequest with reason label
}

// RecordOutputLimitExceeded records an output limit exceeded event (deprecated).
func (m *Metrics) RecordOutputLimitExceeded(action string) {
	// Now handled by RecordRequest with reason label
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
