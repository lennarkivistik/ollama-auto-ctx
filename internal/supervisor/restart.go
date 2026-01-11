package supervisor

import (
	"context"
	"log/slog"
	"os/exec"
	"sync"
	"time"
)

// RestartConfig holds configuration for the restart hook.
type RestartConfig struct {
	Enabled             bool          // SUPERVISOR_RESTART_ENABLED
	Command             string        // SUPERVISOR_RESTART_CMD
	Cooldown            time.Duration // SUPERVISOR_RESTART_COOLDOWN (default 120s)
	MaxPerHour          int           // SUPERVISOR_RESTART_MAX_PER_HOUR (default 3)
	TriggerConsecTimeouts int         // SUPERVISOR_RESTART_TRIGGER_CONSEC_TIMEOUTS (default 2)
	CommandTimeout      time.Duration // SUPERVISOR_RESTART_CMD_TIMEOUT (default 30s)
}

// RestartHook manages Ollama restart operations with safety guards.
// It tracks consecutive timeouts and triggers restarts when thresholds are met.
type RestartHook struct {
	cfg                RestartConfig
	logger             *slog.Logger
	mu                 sync.Mutex
	consecutiveTimeouts int
	restartHistory     []time.Time // timestamps of restarts in the last hour
	lastRestart        time.Time
	restartInProgress  bool
}

// NewRestartHook creates a new restart hook with the given configuration.
func NewRestartHook(cfg RestartConfig, logger *slog.Logger) *RestartHook {
	return &RestartHook{
		cfg:            cfg,
		logger:         logger,
		restartHistory: make([]time.Time, 0, cfg.MaxPerHour+1),
	}
}

// RecordTimeout records a timeout event and potentially triggers a restart.
// Returns true if a restart was triggered.
func (rh *RestartHook) RecordTimeout() bool {
	if !rh.cfg.Enabled || rh.cfg.Command == "" {
		return false
	}

	rh.mu.Lock()
	defer rh.mu.Unlock()

	rh.consecutiveTimeouts++
	
	if rh.consecutiveTimeouts >= rh.cfg.TriggerConsecTimeouts {
		return rh.triggerRestartLocked()
	}
	return false
}

// RecordSuccess records a successful request, resetting the consecutive timeout counter.
func (rh *RestartHook) RecordSuccess() {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	rh.consecutiveTimeouts = 0
}

// triggerRestartLocked attempts to trigger a restart. Caller must hold mutex.
func (rh *RestartHook) triggerRestartLocked() bool {
	// Check if restart is already in progress
	if rh.restartInProgress {
		rh.logger.Debug("restart already in progress, skipping")
		return false
	}

	// Check cooldown
	if time.Since(rh.lastRestart) < rh.cfg.Cooldown {
		rh.logger.Debug("restart cooldown not elapsed",
			"since_last", time.Since(rh.lastRestart),
			"cooldown", rh.cfg.Cooldown)
		return false
	}

	// Check rate limit (max per hour)
	rh.pruneHistoryLocked()
	if len(rh.restartHistory) >= rh.cfg.MaxPerHour {
		rh.logger.Warn("restart rate limit reached",
			"restarts_this_hour", len(rh.restartHistory),
			"max_per_hour", rh.cfg.MaxPerHour)
		return false
	}

	// Mark restart in progress and reset counter
	rh.restartInProgress = true
	rh.consecutiveTimeouts = 0

	// Execute restart asynchronously
	go rh.executeRestart()

	return true
}

// pruneHistoryLocked removes restart entries older than 1 hour. Caller must hold mutex.
func (rh *RestartHook) pruneHistoryLocked() {
	cutoff := time.Now().Add(-time.Hour)
	newHistory := rh.restartHistory[:0]
	for _, t := range rh.restartHistory {
		if t.After(cutoff) {
			newHistory = append(newHistory, t)
		}
	}
	rh.restartHistory = newHistory
}

// executeRestart runs the restart command asynchronously.
func (rh *RestartHook) executeRestart() {
	defer func() {
		// Fail-open: if we panic, mark restart as not in progress
		if r := recover(); r != nil {
			rh.logger.Error("restart panic recovered", "panic", r)
		}
		rh.mu.Lock()
		rh.restartInProgress = false
		rh.mu.Unlock()
	}()

	rh.logger.Info("executing restart command", "command", rh.cfg.Command)

	ctx, cancel := context.WithTimeout(context.Background(), rh.cfg.CommandTimeout)
	defer cancel()

	// Execute via shell
	cmd := exec.CommandContext(ctx, "sh", "-c", rh.cfg.Command)
	output, err := cmd.CombinedOutput()

	rh.mu.Lock()
	rh.lastRestart = time.Now()
	rh.restartHistory = append(rh.restartHistory, rh.lastRestart)
	rh.mu.Unlock()

	if err != nil {
		rh.logger.Error("restart command failed",
			"command", rh.cfg.Command,
			"error", err,
			"output", string(output))
		return
	}

	rh.logger.Info("restart command completed successfully",
		"command", rh.cfg.Command,
		"output", string(output))
}

// GetStats returns current restart hook statistics.
type RestartStats struct {
	ConsecutiveTimeouts int
	RestartsThisHour    int
	LastRestart         *time.Time
	RestartInProgress   bool
}

func (rh *RestartHook) GetStats() RestartStats {
	rh.mu.Lock()
	defer rh.mu.Unlock()

	rh.pruneHistoryLocked()

	stats := RestartStats{
		ConsecutiveTimeouts: rh.consecutiveTimeouts,
		RestartsThisHour:    len(rh.restartHistory),
		RestartInProgress:   rh.restartInProgress,
	}
	if !rh.lastRestart.IsZero() {
		stats.LastRestart = &rh.lastRestart
	}
	return stats
}
