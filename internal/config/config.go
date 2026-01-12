package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// OverridePolicy controls when we overwrite a user-supplied options.num_ctx.
//
// - always:        always overwrite
// - if_missing:    only set when num_ctx is absent
// - if_too_small:  only increase; never decrease
//
// Default is IfTooSmall.
type OverridePolicy string

const (
	OverrideAlways     OverridePolicy = "always"
	OverrideIfMissing  OverridePolicy = "if_missing"
	OverrideIfTooSmall OverridePolicy = "if_too_small"
)

// Config contains all runtime configuration for the proxy.
//
// The proxy is intentionally conservative by default:
// - small contexts stay fast
// - larger contexts are only used when needed
// - upper bounds clamp based on config and model limits
//
// Most users only need to tune MIN_CTX / MAX_CTX and BUCKETS.
type Config struct {
	ListenAddr  string
	UpstreamURL string

	// Context window selection
	MinCtx   int
	MaxCtx   int
	Buckets  []int
	Headroom float64

	// Output token budgeting
	DefaultOutputBudget        int
	MaxOutputBudget            int
	StructuredOverhead         int  // added when format is JSON/schema-like
	DynamicDefaultOutputBudget bool // if true, adjust default budget based on prompt size

	// Estimation overhead defaults (used before auto-calibration converges)
	DefaultFixedOverheadTokens    float64
	DefaultPerMessageOverhead     float64
	DefaultTokensPerByte          float64
	DefaultTokensPerImageFallback int

	OverrideNumCtx OverridePolicy

	// Safety + performance
	RequestBodyMaxBytes       int64
	ResponseTapMaxBytes       int64
	ShowCacheTTL              time.Duration
	CalibrationEnabled        bool
	CalibrationFile           string
	HardwareProbe             bool
	HardwareProbeRefresh      time.Duration
	HardwareProbeVramHeadroom float64

	// HTTP
	CORSAllowOrigin string
	FlushInterval   time.Duration

	// Logging
	LogLevel string

	// System prompt manipulation
	StripSystemPromptText string

	// Supervisor / request tracking (disabled by default)
	SupervisorEnabled       bool
	SupervisorTrackRequests bool
	SupervisorRecentBuffer  int

	// Supervisor / watchdog timeouts (disabled by default)
	SupervisorWatchdogEnabled bool
	SupervisorTTFBTimeout     time.Duration
	SupervisorStallTimeout    time.Duration
	SupervisorHardTimeout     time.Duration

	// Supervisor / observability (disabled by default)
	SupervisorObsEnabled          bool
	SupervisorObsRequestsEndpoint bool
	SupervisorObsSSEEndpoint      bool
	SupervisorObsProgressInterval time.Duration

	// Supervisor / loop detection (disabled by default)
	SupervisorLoopDetectEnabled   bool
	SupervisorLoopWindowBytes     int
	SupervisorLoopNgramBytes      int
	SupervisorLoopRepeatThreshold int
	SupervisorLoopMinOutputBytes  int

	// Supervisor / retry (disabled by default)
	SupervisorRetryEnabled          bool
	SupervisorRetryMaxAttempts      int
	SupervisorRetryBackoff          time.Duration
	SupervisorRetryOnlyNonStreaming bool
	SupervisorRetryMaxResponseBytes int64

	// Supervisor / restart hook (disabled by default)
	SupervisorRestartEnabled             bool
	SupervisorRestartCmd                 string
	SupervisorRestartCooldown            time.Duration
	SupervisorRestartMaxPerHour          int
	SupervisorRestartTriggerConsecTimeouts int
	SupervisorRestartCmdTimeout          time.Duration

	// Supervisor / metrics (disabled by default)
	SupervisorMetricsEnabled bool
	SupervisorMetricsPath    string

	// Supervisor / health check (disabled by default)
	SupervisorHealthCheckEnabled  bool
	SupervisorHealthCheckInterval time.Duration
	SupervisorHealthCheckTimeout  time.Duration

	// Supervisor / output safety limiting (disabled by default)
	SupervisorOutputSafetyLimitEnabled  bool
	SupervisorOutputSafetyLimitTokens   int64
	SupervisorOutputSafetyLimitAction   string // "cancel" or "warn"
}

// Load parses flags/env and returns a validated Config.
func Load() (Config, error) {
	// Defaults (may be overridden by env, then by flags).
	cfg := Config{
		ListenAddr:  getEnvString("LISTEN_ADDR", ":11435"),
		UpstreamURL: getEnvString("UPSTREAM_URL", "http://localhost:11434"),

		MinCtx:   getEnvInt("MIN_CTX", 1024),
		MaxCtx:   getEnvInt("MAX_CTX", 81920),
		Buckets:  getEnvIntList("BUCKETS", []int{1024, 2048, 4096, 8192, 9216, 10240, 11264, 12288, 13312, 14336, 15360, 16384, 20480, 24576, 28672, 32768, 36864, 40960, 45056, 49152, 53248, 57344, 61440, 65536, 69632, 73728, 77824, 81920, 86016, 90112, 94208, 98304, 102400}),
		Headroom: getEnvFloat("HEADROOM", 1.25),

		DefaultOutputBudget:        getEnvInt("DEFAULT_OUTPUT_BUDGET", 1024),
		MaxOutputBudget:            getEnvInt("MAX_OUTPUT_BUDGET", 10240),
		StructuredOverhead:         getEnvInt("STRUCTURED_OVERHEAD", 128),
		DynamicDefaultOutputBudget: getEnvBool("DYNAMIC_DEFAULT_OUTPUT_BUDGET", false),

		DefaultFixedOverheadTokens:    getEnvFloat("DEFAULT_FIXED_OVERHEAD_TOKENS", 32),
		DefaultPerMessageOverhead:     getEnvFloat("DEFAULT_PER_MESSAGE_OVERHEAD_TOKENS", 8),
		DefaultTokensPerByte:          getEnvFloat("DEFAULT_TOKENS_PER_BYTE", 0.25), // ~4 bytes/token
		DefaultTokensPerImageFallback: getEnvInt("DEFAULT_TOKENS_PER_IMAGE", 768),

		OverrideNumCtx: OverridePolicy(getEnvString("OVERRIDE_NUM_CTX", string(OverrideIfTooSmall))),

		RequestBodyMaxBytes: getEnvInt64("REQUEST_BODY_MAX_BYTES", 10*1024*1024), // 10MB
		ResponseTapMaxBytes: getEnvInt64("RESPONSE_TAP_MAX_BYTES", 5*1024*1024),  // 5MB (non-stream JSON)
		ShowCacheTTL:        getEnvDuration("SHOW_CACHE_TTL", 5*time.Minute),
		CalibrationEnabled:  getEnvBool("CALIBRATION_ENABLED", true),
		CalibrationFile:     getEnvString("CALIBRATION_FILE", ""),
		HardwareProbe:       getEnvBool("HARDWARE_PROBE", false),
		HardwareProbeRefresh: getEnvDuration(
			"HARDWARE_PROBE_REFRESH",
			15*time.Second,
		),
		HardwareProbeVramHeadroom: getEnvFloat("HARDWARE_PROBE_VRAM_HEADROOM", 0.15),

		CORSAllowOrigin: getEnvString("CORS_ALLOW_ORIGIN", "*"),
		FlushInterval:   getEnvDuration("FLUSH_INTERVAL", 100*time.Millisecond),

		LogLevel: getEnvString("LOG_LEVEL", "info"),

		StripSystemPromptText: getEnvString("STRIP_SYSTEM_PROMPT_TEXT", ""),

		SupervisorEnabled:       getEnvBool("SUPERVISOR_ENABLED", false),
		SupervisorTrackRequests: getEnvBool("SUPERVISOR_TRACK_REQUESTS", false),
		SupervisorRecentBuffer:  getEnvInt("SUPERVISOR_RECENT_BUFFER", 200),

		SupervisorWatchdogEnabled: getEnvBool("SUPERVISOR_WATCHDOG_ENABLED", false),
		SupervisorTTFBTimeout:     getEnvDuration("SUPERVISOR_TTFB_TIMEOUT", 300*time.Second),
		SupervisorStallTimeout:    getEnvDuration("SUPERVISOR_STALL_TIMEOUT", 200*time.Second),
		SupervisorHardTimeout:     getEnvDuration("SUPERVISOR_HARD_TIMEOUT", 12*time.Minute),

		SupervisorObsEnabled:          getEnvBool("SUPERVISOR_OBS_ENABLED", false),
		SupervisorObsRequestsEndpoint: getEnvBool("SUPERVISOR_OBS_REQUESTS_ENDPOINT", true),
		SupervisorObsSSEEndpoint:      getEnvBool("SUPERVISOR_OBS_SSE_ENDPOINT", true),
		SupervisorObsProgressInterval: getEnvDuration("SUPERVISOR_OBS_PROGRESS_INTERVAL", 250*time.Millisecond),

		SupervisorLoopDetectEnabled:   getEnvBool("SUPERVISOR_LOOP_DETECT_ENABLED", false),
		SupervisorLoopWindowBytes:     getEnvInt("SUPERVISOR_LOOP_WINDOW_BYTES", 4096),
		SupervisorLoopNgramBytes:      getEnvInt("SUPERVISOR_LOOP_NGRAM_BYTES", 64),
		SupervisorLoopRepeatThreshold: getEnvInt("SUPERVISOR_LOOP_REPEAT_THRESHOLD", 3),
		SupervisorLoopMinOutputBytes:  getEnvInt("SUPERVISOR_LOOP_MIN_OUTPUT_BYTES", 1024),

		SupervisorRetryEnabled:          getEnvBool("SUPERVISOR_RETRY_ENABLED", false),
		SupervisorRetryMaxAttempts:      getEnvInt("SUPERVISOR_RETRY_MAX_ATTEMPTS", 2),
		SupervisorRetryBackoff:          getEnvDuration("SUPERVISOR_RETRY_BACKOFF", 250*time.Millisecond),
		SupervisorRetryOnlyNonStreaming: getEnvBool("SUPERVISOR_RETRY_ONLY_NON_STREAMING", true),
		SupervisorRetryMaxResponseBytes: getEnvInt64("SUPERVISOR_RETRY_MAX_RESPONSE_BYTES", 8*1024*1024),

		SupervisorRestartEnabled:             getEnvBool("SUPERVISOR_RESTART_ENABLED", false),
		SupervisorRestartCmd:                 getEnvString("SUPERVISOR_RESTART_CMD", ""),
		SupervisorRestartCooldown:            getEnvDuration("SUPERVISOR_RESTART_COOLDOWN", 120*time.Second),
		SupervisorRestartMaxPerHour:          getEnvInt("SUPERVISOR_RESTART_MAX_PER_HOUR", 3),
		SupervisorRestartTriggerConsecTimeouts: getEnvInt("SUPERVISOR_RESTART_TRIGGER_CONSEC_TIMEOUTS", 2),
		SupervisorRestartCmdTimeout:          getEnvDuration("SUPERVISOR_RESTART_CMD_TIMEOUT", 30*time.Second),

		SupervisorMetricsEnabled: getEnvBool("SUPERVISOR_METRICS_ENABLED", false),
		SupervisorMetricsPath:    getEnvString("SUPERVISOR_METRICS_PATH", "/metrics"),

		SupervisorHealthCheckEnabled:  getEnvBool("SUPERVISOR_HEALTH_CHECK_ENABLED", false),
		SupervisorHealthCheckInterval: getEnvDuration("SUPERVISOR_HEALTH_CHECK_INTERVAL", 30*time.Second),
		SupervisorHealthCheckTimeout:  getEnvDuration("SUPERVISOR_HEALTH_CHECK_TIMEOUT", 5*time.Second),

		SupervisorOutputSafetyLimitEnabled: getEnvBool("SUPERVISOR_OUTPUT_SAFETY_LIMIT_ENABLED", false),
		SupervisorOutputSafetyLimitTokens:  getEnvInt64("SUPERVISOR_OUTPUT_SAFETY_LIMIT_TOKENS", 0),
		SupervisorOutputSafetyLimitAction:   getEnvString("SUPERVISOR_OUTPUT_SAFETY_LIMIT_ACTION", "cancel"),
	}

	// Flags (override env).
	flag.StringVar(&cfg.ListenAddr, "listen", cfg.ListenAddr, "listen address (env LISTEN_ADDR)")
	flag.StringVar(&cfg.UpstreamURL, "upstream", cfg.UpstreamURL, "upstream Ollama base URL (env UPSTREAM_URL)")
	flag.IntVar(&cfg.MinCtx, "min-ctx", cfg.MinCtx, "minimum context size (env MIN_CTX)")
	flag.IntVar(&cfg.MaxCtx, "max-ctx", cfg.MaxCtx, "maximum context size (env MAX_CTX)")
	buckets := flag.String("buckets", intListToString(cfg.Buckets), "comma-separated context buckets (env BUCKETS)")
	flag.Float64Var(&cfg.Headroom, "headroom", cfg.Headroom, "safety headroom multiplier (env HEADROOM)")
	flag.IntVar(&cfg.DefaultOutputBudget, "default-output", cfg.DefaultOutputBudget, "default output token budget if num_predict missing (env DEFAULT_OUTPUT_BUDGET)")
	flag.IntVar(&cfg.MaxOutputBudget, "max-output", cfg.MaxOutputBudget, "max output token budget (env MAX_OUTPUT_BUDGET)")
	flag.IntVar(&cfg.StructuredOverhead, "structured-overhead", cfg.StructuredOverhead, "extra tokens for structured/json outputs (env STRUCTURED_OVERHEAD)")
	flag.BoolVar(&cfg.DynamicDefaultOutputBudget, "dynamic-default-output", cfg.DynamicDefaultOutputBudget, "adjust default output budget based on prompt size (env DYNAMIC_DEFAULT_OUTPUT_BUDGET)")
	flag.StringVar((*string)(&cfg.OverrideNumCtx), "override-policy", string(cfg.OverrideNumCtx), "override policy: always|if_missing|if_too_small (env OVERRIDE_NUM_CTX)")
	flag.Int64Var(&cfg.RequestBodyMaxBytes, "max-body", cfg.RequestBodyMaxBytes, "max request body bytes to parse for rewriting (env REQUEST_BODY_MAX_BYTES)")
	flag.Int64Var(&cfg.ResponseTapMaxBytes, "max-tap", cfg.ResponseTapMaxBytes, "max response bytes to buffer for non-stream calibration (env RESPONSE_TAP_MAX_BYTES)")
	flag.DurationVar(&cfg.ShowCacheTTL, "show-ttl", cfg.ShowCacheTTL, "TTL for /api/show cache (env SHOW_CACHE_TTL)")
	flag.BoolVar(&cfg.CalibrationEnabled, "calibrate", cfg.CalibrationEnabled, "enable auto-calibration using prompt_eval_count (env CALIBRATION_ENABLED)")
	flag.StringVar(&cfg.CalibrationFile, "calibration-file", cfg.CalibrationFile, "optional path to persist calibration JSON (env CALIBRATION_FILE)")
	flag.BoolVar(&cfg.HardwareProbe, "hardware-probe", cfg.HardwareProbe, "best-effort hardware probing (env HARDWARE_PROBE)")
	flag.DurationVar(&cfg.HardwareProbeRefresh, "hardware-probe-refresh", cfg.HardwareProbeRefresh, "hardware probe refresh interval (env HARDWARE_PROBE_REFRESH)")
	flag.Float64Var(&cfg.HardwareProbeVramHeadroom, "hardware-probe-vram-headroom", cfg.HardwareProbeVramHeadroom, "fraction of VRAM to reserve as headroom (env HARDWARE_PROBE_VRAM_HEADROOM)")
	flag.StringVar(&cfg.CORSAllowOrigin, "cors-origin", cfg.CORSAllowOrigin, "CORS allow origin header value (env CORS_ALLOW_ORIGIN)")
	flag.DurationVar(&cfg.FlushInterval, "flush-interval", cfg.FlushInterval, "flush interval for streaming proxy (env FLUSH_INTERVAL)")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "log level: debug|info|warn|error (env LOG_LEVEL)")
	flag.StringVar(&cfg.StripSystemPromptText, "strip-system-prompt", cfg.StripSystemPromptText, "text to remove from system prompts (env STRIP_SYSTEM_PROMPT_TEXT)")
	flag.BoolVar(&cfg.SupervisorEnabled, "supervisor-enabled", cfg.SupervisorEnabled, "enable supervisor layer (env SUPERVISOR_ENABLED)")
	flag.BoolVar(&cfg.SupervisorTrackRequests, "supervisor-track", cfg.SupervisorTrackRequests, "enable request lifecycle tracking (env SUPERVISOR_TRACK_REQUESTS)")
	flag.IntVar(&cfg.SupervisorRecentBuffer, "supervisor-buffer", cfg.SupervisorRecentBuffer, "maximum recent requests to keep in buffer (env SUPERVISOR_RECENT_BUFFER)")
	flag.BoolVar(&cfg.SupervisorWatchdogEnabled, "supervisor-watchdog-enabled", cfg.SupervisorWatchdogEnabled, "enable watchdog timeouts (env SUPERVISOR_WATCHDOG_ENABLED)")
	flag.DurationVar(&cfg.SupervisorTTFBTimeout, "supervisor-ttfb-timeout", cfg.SupervisorTTFBTimeout, "time-to-first-byte timeout (env SUPERVISOR_TTFB_TIMEOUT)")
	flag.DurationVar(&cfg.SupervisorStallTimeout, "supervisor-stall-timeout", cfg.SupervisorStallTimeout, "stall timeout after first byte (env SUPERVISOR_STALL_TIMEOUT)")
	flag.DurationVar(&cfg.SupervisorHardTimeout, "supervisor-hard-timeout", cfg.SupervisorHardTimeout, "hard timeout for total request duration (env SUPERVISOR_HARD_TIMEOUT)")
	flag.BoolVar(&cfg.SupervisorObsEnabled, "supervisor-obs-enabled", cfg.SupervisorObsEnabled, "enable observability endpoints (env SUPERVISOR_OBS_ENABLED)")
	flag.BoolVar(&cfg.SupervisorObsRequestsEndpoint, "supervisor-obs-requests", cfg.SupervisorObsRequestsEndpoint, "enable /debug/requests endpoint (env SUPERVISOR_OBS_REQUESTS_ENDPOINT)")
	flag.BoolVar(&cfg.SupervisorObsSSEEndpoint, "supervisor-obs-sse", cfg.SupervisorObsSSEEndpoint, "enable /events SSE endpoint (env SUPERVISOR_OBS_SSE_ENDPOINT)")
	flag.DurationVar(&cfg.SupervisorObsProgressInterval, "supervisor-obs-progress-interval", cfg.SupervisorObsProgressInterval, "progress event throttling interval (env SUPERVISOR_OBS_PROGRESS_INTERVAL)")
	flag.BoolVar(&cfg.SupervisorLoopDetectEnabled, "supervisor-loop-detect", cfg.SupervisorLoopDetectEnabled, "enable loop detection (env SUPERVISOR_LOOP_DETECT_ENABLED)")
	flag.IntVar(&cfg.SupervisorLoopWindowBytes, "supervisor-loop-window", cfg.SupervisorLoopWindowBytes, "loop detection window size in bytes (env SUPERVISOR_LOOP_WINDOW_BYTES)")
	flag.IntVar(&cfg.SupervisorLoopNgramBytes, "supervisor-loop-ngram", cfg.SupervisorLoopNgramBytes, "loop detection n-gram size in bytes (env SUPERVISOR_LOOP_NGRAM_BYTES)")
	flag.IntVar(&cfg.SupervisorLoopRepeatThreshold, "supervisor-loop-threshold", cfg.SupervisorLoopRepeatThreshold, "loop detection repeat threshold (env SUPERVISOR_LOOP_REPEAT_THRESHOLD)")
	flag.IntVar(&cfg.SupervisorLoopMinOutputBytes, "supervisor-loop-min-output", cfg.SupervisorLoopMinOutputBytes, "minimum output before loop detection activates (env SUPERVISOR_LOOP_MIN_OUTPUT_BYTES)")
	flag.BoolVar(&cfg.SupervisorRetryEnabled, "supervisor-retry-enabled", cfg.SupervisorRetryEnabled, "enable retry logic (env SUPERVISOR_RETRY_ENABLED)")
	flag.IntVar(&cfg.SupervisorRetryMaxAttempts, "supervisor-retry-attempts", cfg.SupervisorRetryMaxAttempts, "max retry attempts (env SUPERVISOR_RETRY_MAX_ATTEMPTS)")
	flag.DurationVar(&cfg.SupervisorRetryBackoff, "supervisor-retry-backoff", cfg.SupervisorRetryBackoff, "retry backoff duration (env SUPERVISOR_RETRY_BACKOFF)")
	flag.BoolVar(&cfg.SupervisorRetryOnlyNonStreaming, "supervisor-retry-non-streaming", cfg.SupervisorRetryOnlyNonStreaming, "only retry non-streaming requests (env SUPERVISOR_RETRY_ONLY_NON_STREAMING)")
	flag.Int64Var(&cfg.SupervisorRetryMaxResponseBytes, "supervisor-retry-max-response", cfg.SupervisorRetryMaxResponseBytes, "max response bytes to buffer for retry (env SUPERVISOR_RETRY_MAX_RESPONSE_BYTES)")
	flag.BoolVar(&cfg.SupervisorRestartEnabled, "supervisor-restart-enabled", cfg.SupervisorRestartEnabled, "enable restart hook (env SUPERVISOR_RESTART_ENABLED)")
	flag.StringVar(&cfg.SupervisorRestartCmd, "supervisor-restart-cmd", cfg.SupervisorRestartCmd, "restart command (env SUPERVISOR_RESTART_CMD)")
	flag.DurationVar(&cfg.SupervisorRestartCooldown, "supervisor-restart-cooldown", cfg.SupervisorRestartCooldown, "restart cooldown duration (env SUPERVISOR_RESTART_COOLDOWN)")
	flag.IntVar(&cfg.SupervisorRestartMaxPerHour, "supervisor-restart-max-hour", cfg.SupervisorRestartMaxPerHour, "max restarts per hour (env SUPERVISOR_RESTART_MAX_PER_HOUR)")
	flag.IntVar(&cfg.SupervisorRestartTriggerConsecTimeouts, "supervisor-restart-trigger", cfg.SupervisorRestartTriggerConsecTimeouts, "consecutive timeouts to trigger restart (env SUPERVISOR_RESTART_TRIGGER_CONSEC_TIMEOUTS)")
	flag.DurationVar(&cfg.SupervisorRestartCmdTimeout, "supervisor-restart-cmd-timeout", cfg.SupervisorRestartCmdTimeout, "restart command timeout (env SUPERVISOR_RESTART_CMD_TIMEOUT)")
	flag.BoolVar(&cfg.SupervisorMetricsEnabled, "supervisor-metrics-enabled", cfg.SupervisorMetricsEnabled, "enable Prometheus metrics endpoint (env SUPERVISOR_METRICS_ENABLED)")
	flag.StringVar(&cfg.SupervisorMetricsPath, "supervisor-metrics-path", cfg.SupervisorMetricsPath, "metrics endpoint path (env SUPERVISOR_METRICS_PATH)")
	flag.BoolVar(&cfg.SupervisorHealthCheckEnabled, "supervisor-health-check-enabled", cfg.SupervisorHealthCheckEnabled, "enable upstream health checking (env SUPERVISOR_HEALTH_CHECK_ENABLED)")
	flag.DurationVar(&cfg.SupervisorHealthCheckInterval, "supervisor-health-check-interval", cfg.SupervisorHealthCheckInterval, "health check interval (env SUPERVISOR_HEALTH_CHECK_INTERVAL)")
	flag.DurationVar(&cfg.SupervisorHealthCheckTimeout, "supervisor-health-check-timeout", cfg.SupervisorHealthCheckTimeout, "health check timeout (env SUPERVISOR_HEALTH_CHECK_TIMEOUT)")
		flag.BoolVar(&cfg.SupervisorOutputSafetyLimitEnabled, "supervisor-output-safety-limit-enabled", cfg.SupervisorOutputSafetyLimitEnabled, "enable output safety limiting (env SUPERVISOR_OUTPUT_SAFETY_LIMIT_ENABLED)")
		flag.Int64Var(&cfg.SupervisorOutputSafetyLimitTokens, "supervisor-output-safety-limit-tokens", cfg.SupervisorOutputSafetyLimitTokens, "maximum output tokens before safety limit (0 = disabled) (env SUPERVISOR_OUTPUT_SAFETY_LIMIT_TOKENS)")
		flag.StringVar(&cfg.SupervisorOutputSafetyLimitAction, "supervisor-output-safety-limit-action", cfg.SupervisorOutputSafetyLimitAction, "action when safety limit exceeded: cancel or warn (env SUPERVISOR_OUTPUT_SAFETY_LIMIT_ACTION)")

	flag.Parse()

	// Parse buckets flag.
	if buckets != nil {
		parsed, err := parseIntList(*buckets)
		if err != nil {
			return Config{}, fmt.Errorf("invalid buckets: %w", err)
		}
		if len(parsed) > 0 {
			cfg.Buckets = parsed
		}
	}

	// Auto-enable supervisor monitoring features when SUPERVISOR_ENABLED=true
	// Only apply if the user hasn't explicitly set these environment variables
	if cfg.SupervisorEnabled {
		// Auto-enable request tracking (required for most supervisor features)
		if _, trackSet := os.LookupEnv("SUPERVISOR_TRACK_REQUESTS"); !trackSet {
			cfg.SupervisorTrackRequests = true
		}

		// Auto-enable observability (for monitoring and dashboard)
		if _, obsSet := os.LookupEnv("SUPERVISOR_OBS_ENABLED"); !obsSet {
			cfg.SupervisorObsEnabled = true
		}

		// Auto-enable metrics (Prometheus endpoint)
		if _, metricsSet := os.LookupEnv("SUPERVISOR_METRICS_ENABLED"); !metricsSet {
			cfg.SupervisorMetricsEnabled = true
		}

		// Auto-enable health checking (used by dashboard and monitoring)
		if _, healthSet := os.LookupEnv("SUPERVISOR_HEALTH_CHECK_ENABLED"); !healthSet {
			cfg.SupervisorHealthCheckEnabled = true
		}
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.MinCtx <= 0 {
		return fmt.Errorf("MIN_CTX must be > 0")
	}
	if c.MaxCtx <= 0 {
		return fmt.Errorf("MAX_CTX must be > 0")
	}
	if c.MinCtx > c.MaxCtx {
		return fmt.Errorf("MIN_CTX must be <= MAX_CTX")
	}
	if c.Headroom < 1.0 {
		return fmt.Errorf("HEADROOM must be >= 1.0")
	}
	if c.DefaultOutputBudget < 0 || c.MaxOutputBudget < 0 {
		return fmt.Errorf("output budgets must be >= 0")
	}
	if c.DefaultOutputBudget > c.MaxOutputBudget {
		return fmt.Errorf("DEFAULT_OUTPUT_BUDGET must be <= MAX_OUTPUT_BUDGET")
	}
	if c.SupervisorRecentBuffer < 0 {
		return fmt.Errorf("SUPERVISOR_RECENT_BUFFER must be >= 0")
	}
	if c.SupervisorTTFBTimeout <= 0 {
		return fmt.Errorf("SUPERVISOR_TTFB_TIMEOUT must be > 0")
	}
	if c.SupervisorStallTimeout <= 0 {
		return fmt.Errorf("SUPERVISOR_STALL_TIMEOUT must be > 0")
	}
	if c.SupervisorHardTimeout <= 0 {
		return fmt.Errorf("SUPERVISOR_HARD_TIMEOUT must be > 0")
	}
	if c.SupervisorObsProgressInterval <= 0 {
		return fmt.Errorf("SUPERVISOR_OBS_PROGRESS_INTERVAL must be > 0")
	}
	if c.SupervisorLoopWindowBytes < 256 {
		return fmt.Errorf("SUPERVISOR_LOOP_WINDOW_BYTES must be >= 256")
	}
	if c.SupervisorLoopNgramBytes < 8 {
		return fmt.Errorf("SUPERVISOR_LOOP_NGRAM_BYTES must be >= 8")
	}
	if c.SupervisorLoopRepeatThreshold < 2 {
		return fmt.Errorf("SUPERVISOR_LOOP_REPEAT_THRESHOLD must be >= 2")
	}
	if c.SupervisorLoopMinOutputBytes < 256 {
		return fmt.Errorf("SUPERVISOR_LOOP_MIN_OUTPUT_BYTES must be >= 256")
	}
	if c.SupervisorRetryMaxAttempts < 1 {
		return fmt.Errorf("SUPERVISOR_RETRY_MAX_ATTEMPTS must be >= 1")
	}
	if c.SupervisorRetryBackoff < 0 {
		return fmt.Errorf("SUPERVISOR_RETRY_BACKOFF must be >= 0")
	}
	if c.SupervisorRetryMaxResponseBytes < 0 {
		return fmt.Errorf("SUPERVISOR_RETRY_MAX_RESPONSE_BYTES must be >= 0")
	}
	if c.SupervisorRestartEnabled && c.SupervisorRestartCmd == "" {
		return fmt.Errorf("SUPERVISOR_RESTART_CMD is required when SUPERVISOR_RESTART_ENABLED is true")
	}
	if c.SupervisorRestartCooldown < 0 {
		return fmt.Errorf("SUPERVISOR_RESTART_COOLDOWN must be >= 0")
	}
	if c.SupervisorRestartMaxPerHour < 1 {
		return fmt.Errorf("SUPERVISOR_RESTART_MAX_PER_HOUR must be >= 1")
	}
	if c.SupervisorRestartTriggerConsecTimeouts < 1 {
		return fmt.Errorf("SUPERVISOR_RESTART_TRIGGER_CONSEC_TIMEOUTS must be >= 1")
	}
	if c.SupervisorRestartCmdTimeout <= 0 {
		return fmt.Errorf("SUPERVISOR_RESTART_CMD_TIMEOUT must be > 0")
	}
	if c.SupervisorHealthCheckInterval <= 0 {
		return fmt.Errorf("SUPERVISOR_HEALTH_CHECK_INTERVAL must be > 0")
	}
	if c.SupervisorHealthCheckTimeout <= 0 {
		return fmt.Errorf("SUPERVISOR_HEALTH_CHECK_TIMEOUT must be > 0")
	}
	if c.SupervisorOutputSafetyLimitTokens < 0 {
		return fmt.Errorf("SUPERVISOR_OUTPUT_SAFETY_LIMIT_TOKENS must be >= 0")
	}
	if c.SupervisorOutputSafetyLimitAction != "cancel" && c.SupervisorOutputSafetyLimitAction != "warn" {
		return fmt.Errorf("SUPERVISOR_OUTPUT_SAFETY_LIMIT_ACTION must be 'cancel' or 'warn'")
	}
	switch c.OverrideNumCtx {
	case OverrideAlways, OverrideIfMissing, OverrideIfTooSmall:
		// ok
	default:
		return fmt.Errorf("invalid OVERRIDE_NUM_CTX: %q", c.OverrideNumCtx)
	}
	if len(c.Buckets) == 0 {
		return fmt.Errorf("BUCKETS must not be empty")
	}
	prev := 0
	for _, b := range c.Buckets {
		if b <= 0 {
			return fmt.Errorf("BUCKETS values must be > 0")
		}
		if b < prev {
			return fmt.Errorf("BUCKETS must be ascending")
		}
		prev = b
	}
	return nil
}

func getEnvString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func getEnvInt64(key string, def int64) int64 {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return n
		}
	}
	return def
}

func getEnvFloat(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			return d
		}
	}
	return def
}

func getEnvIntList(key string, def []int) []int {
	if v, ok := os.LookupEnv(key); ok {
		if parsed, err := parseIntList(v); err == nil && len(parsed) > 0 {
			return parsed
		}
	}
	return def
}

func parseIntList(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func intListToString(xs []int) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, strconv.Itoa(x))
	}
	return strings.Join(parts, ",")
}
