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

	h := proxy.NewHandler(cfg, ollamaClient.BaseURL, showCache, calibStore, logger)

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
	)
}
