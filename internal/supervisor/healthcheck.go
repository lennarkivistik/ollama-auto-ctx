package supervisor

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// HealthChecker periodically checks Ollama upstream health.
type HealthChecker struct {
	upstreamURL   string
	checkInterval time.Duration
	timeout       time.Duration
	healthy       atomic.Bool
	lastCheck     atomic.Value // time.Time
	lastError     atomic.Value // string
	metrics       *Metrics
	logger        *slog.Logger
	client        *http.Client
	stopCh        chan struct{}
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(upstreamURL string, checkInterval, timeout time.Duration, metrics *Metrics, logger *slog.Logger) *HealthChecker {
	hc := &HealthChecker{
		upstreamURL:   upstreamURL,
		checkInterval: checkInterval,
		timeout:       timeout,
		metrics:       metrics,
		logger:        logger,
		client: &http.Client{
			Timeout: timeout,
		},
		stopCh: make(chan struct{}),
	}

	// Initialize as unhealthy until first check
	hc.healthy.Store(false)

	// Start background checker
	go hc.run()

	return hc
}

// run performs periodic health checks.
func (hc *HealthChecker) run() {
	// Perform initial check immediately
	hc.check()

	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.check()
		case <-hc.stopCh:
			return
		}
	}
}

// check performs a single health check by pinging /api/tags.
func (hc *HealthChecker) check() {
	ctx, cancel := context.WithTimeout(context.Background(), hc.timeout)
	defer cancel()

	url := hc.upstreamURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		hc.updateHealth(false, err.Error())
		return
	}

	resp, err := hc.client.Do(req)
	if err != nil {
		hc.updateHealth(false, err.Error())
		return
	}
	defer resp.Body.Close()

	// Consider 2xx and 3xx as healthy
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		hc.updateHealth(true, "")
	} else {
		hc.updateHealth(false, "status code: "+string(rune(resp.StatusCode)))
	}
}

// updateHealth updates the health status and metrics.
func (hc *HealthChecker) updateHealth(healthy bool, errMsg string) {
	hc.healthy.Store(healthy)
	hc.lastCheck.Store(time.Now())
	if errMsg != "" {
		hc.lastError.Store(errMsg)
		if hc.logger != nil {
			hc.logger.Debug("upstream health check failed", "error", errMsg)
		}
	} else {
		hc.lastError.Store("")
	}

	// Update metrics if available
	if hc.metrics != nil {
		hc.metrics.UpdateUpstreamHealth(healthy)
	}
}

// Healthy returns whether the upstream is currently healthy.
func (hc *HealthChecker) Healthy() bool {
	return hc.healthy.Load()
}

// LastCheck returns the time of the last health check.
func (hc *HealthChecker) LastCheck() time.Time {
	if v := hc.lastCheck.Load(); v != nil {
		return v.(time.Time)
	}
	return time.Time{}
}

// LastError returns the last error message, if any.
func (hc *HealthChecker) LastError() string {
	if v := hc.lastError.Load(); v != nil {
		return v.(string)
	}
	return ""
}

// Shutdown stops the health checker.
func (hc *HealthChecker) Shutdown() {
	close(hc.stopCh)
}
