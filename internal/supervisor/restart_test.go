package supervisor

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestRestartHook_Disabled(t *testing.T) {
	cfg := RestartConfig{
		Enabled: false,
	}

	hook := NewRestartHook(cfg, slog.Default())

	if hook.RecordTimeout() {
		t.Errorf("disabled hook should not trigger restart")
	}
}

func TestRestartHook_EmptyCommand(t *testing.T) {
	cfg := RestartConfig{
		Enabled:               true,
		Command:               "", // Empty command
		TriggerConsecTimeouts: 1,
	}

	hook := NewRestartHook(cfg, slog.Default())

	if hook.RecordTimeout() {
		t.Errorf("hook with empty command should not trigger restart")
	}
}

func TestRestartHook_ConsecutiveTimeoutsThreshold(t *testing.T) {
	cfg := RestartConfig{
		Enabled:               true,
		Command:               "echo test",
		Cooldown:              0,
		MaxPerHour:            10,
		TriggerConsecTimeouts: 3,
		CommandTimeout:        time.Second,
	}

	hook := NewRestartHook(cfg, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	// First timeout - should not trigger
	if hook.RecordTimeout() {
		t.Errorf("should not trigger after 1 timeout")
	}

	// Second timeout - should not trigger
	if hook.RecordTimeout() {
		t.Errorf("should not trigger after 2 timeouts")
	}

	// Third timeout - should trigger
	if !hook.RecordTimeout() {
		t.Errorf("should trigger after 3 timeouts")
	}

	// Wait for restart to complete
	time.Sleep(100 * time.Millisecond)

	stats := hook.GetStats()
	if stats.ConsecutiveTimeouts != 0 {
		t.Errorf("consecutive timeouts should be reset after restart")
	}
}

func TestRestartHook_SuccessResetsCounter(t *testing.T) {
	cfg := RestartConfig{
		Enabled:               true,
		Command:               "echo test",
		TriggerConsecTimeouts: 3,
	}

	hook := NewRestartHook(cfg, slog.Default())

	// Two timeouts
	hook.RecordTimeout()
	hook.RecordTimeout()

	stats := hook.GetStats()
	if stats.ConsecutiveTimeouts != 2 {
		t.Errorf("expected 2 consecutive timeouts, got %d", stats.ConsecutiveTimeouts)
	}

	// Success resets counter
	hook.RecordSuccess()

	stats = hook.GetStats()
	if stats.ConsecutiveTimeouts != 0 {
		t.Errorf("consecutive timeouts should be reset after success")
	}
}

func TestRestartHook_Cooldown(t *testing.T) {
	cfg := RestartConfig{
		Enabled:               true,
		Command:               "echo test",
		Cooldown:              time.Hour, // Long cooldown
		MaxPerHour:            10,
		TriggerConsecTimeouts: 1,
		CommandTimeout:        time.Second,
	}

	hook := NewRestartHook(cfg, slog.Default())

	// First restart should trigger
	if !hook.RecordTimeout() {
		t.Errorf("first restart should trigger")
	}

	// Wait for restart to complete
	time.Sleep(100 * time.Millisecond)

	// Second restart should be blocked by cooldown
	if hook.RecordTimeout() {
		t.Errorf("second restart should be blocked by cooldown")
	}
}

func TestRestartHook_RateLimit(t *testing.T) {
	cfg := RestartConfig{
		Enabled:               true,
		Command:               "echo test",
		Cooldown:              0, // No cooldown
		MaxPerHour:            2, // Only 2 per hour
		TriggerConsecTimeouts: 1,
		CommandTimeout:        time.Second,
	}

	hook := NewRestartHook(cfg, slog.Default())

	// First restart
	if !hook.RecordTimeout() {
		t.Errorf("first restart should trigger")
	}
	time.Sleep(100 * time.Millisecond)

	// Second restart
	if !hook.RecordTimeout() {
		t.Errorf("second restart should trigger")
	}
	time.Sleep(100 * time.Millisecond)

	// Third restart should be blocked by rate limit
	if hook.RecordTimeout() {
		t.Errorf("third restart should be blocked by rate limit")
	}

	stats := hook.GetStats()
	if stats.RestartsThisHour != 2 {
		t.Errorf("expected 2 restarts this hour, got %d", stats.RestartsThisHour)
	}
}

func TestRestartHook_NoDoubleRestart(t *testing.T) {
	cfg := RestartConfig{
		Enabled:               true,
		Command:               "sleep 0.5", // Takes some time
		Cooldown:              0,
		MaxPerHour:            10,
		TriggerConsecTimeouts: 1,
		CommandTimeout:        time.Second,
	}

	hook := NewRestartHook(cfg, slog.Default())

	// First restart triggers
	if !hook.RecordTimeout() {
		t.Errorf("first restart should trigger")
	}

	// Immediate second restart should be blocked (restart in progress)
	if hook.RecordTimeout() {
		t.Errorf("second restart should be blocked while first is in progress")
	}

	stats := hook.GetStats()
	if !stats.RestartInProgress {
		t.Errorf("restart should be marked as in progress")
	}

	// Wait for restart to complete
	time.Sleep(600 * time.Millisecond)

	stats = hook.GetStats()
	if stats.RestartInProgress {
		t.Errorf("restart should be marked as complete")
	}
}
