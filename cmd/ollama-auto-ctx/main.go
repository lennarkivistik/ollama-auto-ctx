package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ollama-auto-ctx/internal/calibration"
	"ollama-auto-ctx/internal/config"
	"ollama-auto-ctx/internal/ollama"
	"ollama-auto-ctx/internal/proxy"
	"ollama-auto-ctx/internal/supervisor"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(2)
	}

	logger := newLogger(cfg.LogLevel)

	logConfig(logger, cfg)

	ollamaClient, err := ollama.NewClient(cfg.UpstreamURL)
	if err != nil {
		logger.Error("failed to create ollama client", "err", err)
		os.Exit(2)
	}

	showCache := ollama.NewShowCache(ollamaClient, cfg.ShowCacheTTL)

	defaults := calibration.Params{
		TokensPerByte:      cfg.DefaultTokensPerByte,
		FixedOverhead:      cfg.DefaultFixedOverheadTokens,
		PerMessageOverhead: cfg.DefaultPerMessageOverhead,
	}
	calibStore := calibration.NewStore(0.20, defaults, cfg.CalibrationFile)

	var tracker *supervisor.Tracker
	var watchdog *supervisor.Watchdog
	var eventBus *supervisor.EventBus
	var restartHook *supervisor.RestartHook
	var retryer *supervisor.Retryer
	var metrics *supervisor.Metrics
	var healthChecker *supervisor.HealthChecker
	if cfg.SupervisorEnabled {
		// Create metrics if enabled
		if cfg.SupervisorMetricsEnabled {
			metrics = supervisor.NewMetrics()
		}

		// Create event bus if observability is enabled
		if cfg.SupervisorObsEnabled {
			eventBus = supervisor.NewEventBus(100) // bounded buffer
		}

		tracker = supervisor.NewTracker(cfg.SupervisorRecentBuffer, eventBus, calibStore, cfg.DefaultTokensPerByte, cfg.SupervisorObsProgressInterval, metrics)

		// Create restart hook if enabled
		if cfg.SupervisorRestartEnabled {
			restartHook = supervisor.NewRestartHook(supervisor.RestartConfig{
				Enabled:               cfg.SupervisorRestartEnabled,
				Command:               cfg.SupervisorRestartCmd,
				Cooldown:              cfg.SupervisorRestartCooldown,
				MaxPerHour:            cfg.SupervisorRestartMaxPerHour,
				TriggerConsecTimeouts: cfg.SupervisorRestartTriggerConsecTimeouts,
				CommandTimeout:        cfg.SupervisorRestartCmdTimeout,
			}, logger)
		}

		if cfg.SupervisorWatchdogEnabled {
			watchdog = supervisor.NewWatchdog(tracker, cfg.SupervisorTTFBTimeout, cfg.SupervisorStallTimeout, cfg.SupervisorHardTimeout, logger, restartHook)
			go watchdog.Run()
			defer watchdog.Shutdown()
		}

		// Create retryer if enabled
		if cfg.SupervisorRetryEnabled {
			retryer = supervisor.NewRetryer(supervisor.RetryConfig{
				Enabled:          cfg.SupervisorRetryEnabled,
				MaxAttempts:      cfg.SupervisorRetryMaxAttempts,
				Backoff:          cfg.SupervisorRetryBackoff,
				OnlyNonStreaming: cfg.SupervisorRetryOnlyNonStreaming,
				MaxResponseBytes: cfg.SupervisorRetryMaxResponseBytes,
			})
		}

		// Create health checker if enabled
		if cfg.SupervisorHealthCheckEnabled {
			healthChecker = supervisor.NewHealthChecker(
				cfg.UpstreamURL,
				cfg.SupervisorHealthCheckInterval,
				cfg.SupervisorHealthCheckTimeout,
				metrics,
				logger,
			)
			defer healthChecker.Shutdown()
		}
	}

	h := proxy.NewHandler(cfg, ollamaClient.BaseURL, showCache, calibStore, tracker, watchdog, eventBus, retryer, metrics, healthChecker, logger)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	logger.Info("starting ollama-auto-ctx", "listen", cfg.ListenAddr, "upstream", cfg.UpstreamURL)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func newLogger(level string) *slog.Logger {
	lvl := new(slog.LevelVar)
	switch level {
	case "debug":
		lvl.Set(slog.LevelDebug)
	case "info":
		lvl.Set(slog.LevelInfo)
	case "warn", "warning":
		lvl.Set(slog.LevelWarn)
	case "error":
		lvl.Set(slog.LevelError)
	default:
		lvl.Set(slog.LevelInfo)
	}

	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(h)
}

func logConfig(logger *slog.Logger, cfg config.Config) {
	// Format buckets as comma-separated string for readability
	bucketStrs := make([]string, len(cfg.Buckets))
	for i, b := range cfg.Buckets {
		bucketStrs[i] = strconv.Itoa(b)
	}
	bucketsStr := strings.Join(bucketStrs, ",")

	logger.Info("configuration",
		"listen_addr", cfg.ListenAddr,
		"upstream_url", cfg.UpstreamURL,
		"min_ctx", cfg.MinCtx,
		"max_ctx", cfg.MaxCtx,
		"buckets", bucketsStr,
		"headroom", cfg.Headroom,
		"default_output_budget", cfg.DefaultOutputBudget,
		"max_output_budget", cfg.MaxOutputBudget,
		"structured_overhead", cfg.StructuredOverhead,
		"dynamic_default_output_budget", cfg.DynamicDefaultOutputBudget,
		"override_policy", string(cfg.OverrideNumCtx),
		"calibration_enabled", cfg.CalibrationEnabled,
		"calibration_file", cfg.CalibrationFile,
		"show_cache_ttl", cfg.ShowCacheTTL,
		"request_body_max_bytes", cfg.RequestBodyMaxBytes,
		"response_tap_max_bytes", cfg.ResponseTapMaxBytes,
		"cors_allow_origin", cfg.CORSAllowOrigin,
		"flush_interval", cfg.FlushInterval,
		"log_level", cfg.LogLevel,
		"supervisor_enabled", cfg.SupervisorEnabled,
		"supervisor_track_requests", cfg.SupervisorTrackRequests,
		"supervisor_recent_buffer", cfg.SupervisorRecentBuffer,
		"supervisor_watchdog_enabled", cfg.SupervisorWatchdogEnabled,
		"supervisor_ttfb_timeout", cfg.SupervisorTTFBTimeout,
		"supervisor_stall_timeout", cfg.SupervisorStallTimeout,
		"supervisor_hard_timeout", cfg.SupervisorHardTimeout,
		"supervisor_obs_enabled", cfg.SupervisorObsEnabled,
		"supervisor_obs_requests_endpoint", cfg.SupervisorObsRequestsEndpoint,
		"supervisor_obs_sse_endpoint", cfg.SupervisorObsSSEEndpoint,
		"supervisor_obs_progress_interval", cfg.SupervisorObsProgressInterval,
		"supervisor_loop_detect_enabled", cfg.SupervisorLoopDetectEnabled,
		"supervisor_loop_window_bytes", cfg.SupervisorLoopWindowBytes,
		"supervisor_loop_ngram_bytes", cfg.SupervisorLoopNgramBytes,
		"supervisor_loop_repeat_threshold", cfg.SupervisorLoopRepeatThreshold,
		"supervisor_loop_min_output_bytes", cfg.SupervisorLoopMinOutputBytes,
		"supervisor_retry_enabled", cfg.SupervisorRetryEnabled,
		"supervisor_retry_max_attempts", cfg.SupervisorRetryMaxAttempts,
		"supervisor_retry_backoff", cfg.SupervisorRetryBackoff,
		"supervisor_retry_only_non_streaming", cfg.SupervisorRetryOnlyNonStreaming,
		"supervisor_retry_max_response_bytes", cfg.SupervisorRetryMaxResponseBytes,
		"supervisor_restart_enabled", cfg.SupervisorRestartEnabled,
		"supervisor_restart_cmd", cfg.SupervisorRestartCmd,
		"supervisor_restart_cooldown", cfg.SupervisorRestartCooldown,
		"supervisor_restart_max_per_hour", cfg.SupervisorRestartMaxPerHour,
		"supervisor_restart_trigger_consec_timeouts", cfg.SupervisorRestartTriggerConsecTimeouts,
		"supervisor_restart_cmd_timeout", cfg.SupervisorRestartCmdTimeout,
		"supervisor_metrics_enabled", cfg.SupervisorMetricsEnabled,
		"supervisor_metrics_path", cfg.SupervisorMetricsPath,
		"supervisor_health_check_enabled", cfg.SupervisorHealthCheckEnabled,
		"supervisor_health_check_interval", cfg.SupervisorHealthCheckInterval,
		"supervisor_health_check_timeout", cfg.SupervisorHealthCheckTimeout,
		"supervisor_output_limit_enabled", cfg.SupervisorOutputLimitEnabled,
		"supervisor_output_limit_tokens", cfg.SupervisorOutputLimitTokens,
		"supervisor_output_limit_action", cfg.SupervisorOutputLimitAction,
	)
}
