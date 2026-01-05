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
