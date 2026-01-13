package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ollama-auto-ctx/internal/api"
	"ollama-auto-ctx/internal/calibration"
	"ollama-auto-ctx/internal/config"
	"ollama-auto-ctx/internal/ollama"
	"ollama-auto-ctx/internal/proxy"
	"ollama-auto-ctx/internal/storage"
	"ollama-auto-ctx/internal/supervisor"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(2)
	}

	logger := newLogger(cfg.LogLevel)
	features := cfg.Features()

	logConfig(logger, cfg, features)

	// Create Ollama client
	ollamaClient, err := ollama.NewClient(cfg.UpstreamURL)
	if err != nil {
		logger.Error("failed to create ollama client", "err", err)
		os.Exit(2)
	}

	showCache := ollama.NewShowCache(ollamaClient, cfg.ShowCacheTTL)

	// Calibration store
	defaults := calibration.Params{
		TokensPerByte:      cfg.DefaultTokensPerByte,
		FixedOverhead:      cfg.DefaultFixedOverheadTokens,
		PerMessageOverhead: cfg.DefaultPerMessageOverhead,
	}
	calibStore := calibration.NewStore(0.20, defaults, cfg.CalibrationFile)

	// Storage (SQLite or Memory)
	var store storage.Store
	if features.Storage {
		switch cfg.Storage {
		case config.StorageSQLite:
			store, err = storage.NewSQLiteStore(cfg.StoragePath, cfg.StorageMaxRows, logger)
			if err != nil {
				logger.Warn("SQLite storage not available, falling back to memory", "err", err)
				store = storage.NewMemoryStore(cfg.StorageMaxRows)
			}
		case config.StorageMemory:
			store = storage.NewMemoryStore(cfg.StorageMaxRows)
		default:
			// Auto-detect: try SQLite first, fall back to memory
			store, err = storage.NewSQLiteStore(cfg.StoragePath, cfg.StorageMaxRows, logger)
			if err != nil {
				logger.Debug("SQLite not available, using memory storage", "err", err)
				store = storage.NewMemoryStore(cfg.StorageMaxRows)
			}
		}
		if store != nil {
			defer store.Close()
		}
	}

	// API server
	var apiServer *api.Server
	if features.API && store != nil {
		apiServer = api.NewServer(store, cfg, logger)
	}

	// Supervisor components (legacy compatibility, used when features.Protect is true)
	var tracker *supervisor.Tracker
	var watchdog *supervisor.Watchdog
	var eventBus *supervisor.EventBus
	var restartHook *supervisor.RestartHook
	var retryer *supervisor.Retryer
	var metrics *supervisor.Metrics
	var healthChecker *supervisor.HealthChecker

	// Only create supervisor components if we have storage (for now) and features enabled
	if features.Events || features.Metrics || features.Retry || features.Protect {
		// Create metrics if enabled
		if features.Metrics {
			metrics = supervisor.NewMetrics()
		}

		// Create event bus if enabled
		if features.Events {
			eventBus = supervisor.NewEventBus(100)
		}

		// Create tracker
		tracker = supervisor.NewTracker(
			cfg.RecentBuffer,
			eventBus,
			calibStore,
			cfg.DefaultTokensPerByte,
			cfg.ProgressInterval,
			metrics,
		)

		// Create retryer if retry mode enabled
		if features.Retry {
			retryer = supervisor.NewRetryer(supervisor.RetryConfig{
				Enabled:          true,
				MaxAttempts:      cfg.RetryMax,
				Backoff:          time.Duration(cfg.RetryBackoffMs) * time.Millisecond,
				OnlyNonStreaming: true,
				MaxResponseBytes: 8 * 1024 * 1024,
			})
		}

		// Protect mode features
		if features.Protect {
			// Watchdog
			watchdog = supervisor.NewWatchdog(
				tracker,
				time.Duration(cfg.TimeoutTTFBMs)*time.Millisecond,
				time.Duration(cfg.TimeoutStallMs)*time.Millisecond,
				time.Duration(cfg.TimeoutHardMs)*time.Millisecond,
				logger,
				restartHook,
			)
			go watchdog.Run()
			defer watchdog.Shutdown()
		}

		// Health checker
		healthChecker = supervisor.NewHealthChecker(
			cfg.UpstreamURL,
			cfg.HealthCheckInterval,
			cfg.HealthCheckTimeout,
			metrics,
			logger,
		)
		defer healthChecker.Shutdown()
	}

	// Create handler
	h := proxy.NewHandler(
		cfg,
		features,
		ollamaClient.BaseURL,
		showCache,
		calibStore,
		store,
		apiServer,
		tracker,
		watchdog,
		eventBus,
		retryer,
		metrics,
		healthChecker,
		logger,
	)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	logger.Info("starting ollama-auto-ctx",
		"listen", cfg.ListenAddr,
		"upstream", cfg.UpstreamURL,
		"mode", cfg.Mode,
	)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
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

func logConfig(logger *slog.Logger, cfg config.Config, f config.Features) {
	logger.Info("configuration",
		"mode", cfg.Mode,
		"listen_addr", cfg.ListenAddr,
		"upstream_url", cfg.UpstreamURL,
		"storage", cfg.Storage,
		"storage_path", cfg.StoragePath,
		"storage_max_rows", cfg.StorageMaxRows,
		"features.dashboard", f.Dashboard,
		"features.api", f.API,
		"features.events", f.Events,
		"features.metrics", f.Metrics,
		"features.storage", f.Storage,
		"features.retry", f.Retry,
		"features.protect", f.Protect,
		"min_ctx", cfg.MinCtx,
		"max_ctx", cfg.MaxCtx,
		"headroom", cfg.Headroom,
		"calibration_enabled", cfg.CalibrationEnabled,
	)
}
