package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Mode controls which features are enabled.
// - off: proxy + AutoCTX only, no dashboard/metrics/storage
// - monitor: dashboard + events + sqlite + api + metrics (no retry, no protect)
// - retry (default): monitor + retry logic
// - protect: retry + watchdog timeouts + loop detect + output limit
type Mode string

const (
	ModeOff     Mode = "off"
	ModeMonitor Mode = "monitor"
	ModeRetry   Mode = "retry"   // default
	ModeProtect Mode = "protect"
)

// StorageType controls the storage backend.
type StorageType string

const (
	StorageSQLite StorageType = "sqlite"
	StorageMemory StorageType = "memory"
	StorageOff    StorageType = "off"
)

// OverridePolicy controls when we overwrite a user-supplied options.num_ctx.
type OverridePolicy string

const (
	OverrideAlways     OverridePolicy = "always"
	OverrideIfMissing  OverridePolicy = "if_missing"
	OverrideIfTooSmall OverridePolicy = "if_too_small"
)

// Features derived from MODE - centralized feature gating.
type Features struct {
	Dashboard bool
	API       bool
	Events    bool
	Metrics   bool
	Storage   bool
	Retry     bool
	Protect   bool
}

// Config contains all runtime configuration for the proxy.
type Config struct {
	// Core
	Mode        Mode
	ListenAddr  string
	UpstreamURL string
	LogLevel    string

	// Storage (enabled when MODE != off unless explicitly disabled)
	Storage        StorageType
	StoragePath    string
	StorageMaxRows int

	// Retry (enabled when MODE in retry/protect)
	RetryMax       int
	RetryBackoffMs int

	// Protect (enabled only when MODE=protect)
	TimeoutTTFBMs        int
	TimeoutStallMs       int
	TimeoutHardMs        int
	LoopDetectEnabled    bool
	LoopWindowBytes      int
	LoopNgramBytes       int
	LoopRepeatThreshold  int
	LoopMinOutputBytes   int
	OutputLimitEnabled   bool
	OutputLimitMaxTokens int

	// Context window selection (always on)
	MinCtx   int
	MaxCtx   int
	Buckets  []int
	Headroom float64

	// Output token budgeting
	DefaultOutputBudget        int
	MaxOutputBudget            int
	StructuredOverhead         int
	DynamicDefaultOutputBudget bool

	// Estimation overhead defaults
	DefaultFixedOverheadTokens    float64
	DefaultPerMessageOverhead     float64
	DefaultTokensPerByte          float64
	DefaultTokensPerImageFallback int

	OverrideNumCtx OverridePolicy

	// Safety + performance
	RequestBodyMaxBytes  int64
	ResponseTapMaxBytes  int64
	ShowCacheTTL         time.Duration
	CalibrationEnabled   bool
	CalibrationFile      string
	ProgressInterval     time.Duration
	RecentBuffer         int
	HealthCheckInterval  time.Duration
	HealthCheckTimeout   time.Duration

	// HTTP
	CORSAllowOrigin string
	FlushInterval   time.Duration

	// System prompt manipulation
	StripSystemPromptText string
}

// Features returns the feature flags derived from the current MODE.
// This is the central place for feature gating.
func (c *Config) Features() Features {
	if c.Mode == ModeOff {
		return Features{} // all false
	}

	f := Features{
		Dashboard: true,
		API:       true,
		Events:    true,
		Metrics:   true,
		Storage:   c.Storage != StorageOff,
	}

	if c.Mode == ModeRetry || c.Mode == ModeProtect {
		f.Retry = true
	}

	if c.Mode == ModeProtect {
		f.Protect = true
	}

	return f
}

// Load parses env vars and returns a validated Config.
func Load() (Config, error) {
	// Parse MODE first as it affects defaults
	mode := Mode(getEnvString("MODE", string(ModeRetry)))

	// Storage defaults based on mode
	var storageDefault StorageType
	if mode == ModeOff {
		storageDefault = StorageOff
	} else {
		storageDefault = StorageSQLite
	}

	cfg := Config{
		// Core
		Mode:        mode,
		ListenAddr:  getEnvString("LISTEN_ADDR", ":11435"),
		UpstreamURL: getEnvString("UPSTREAM_URL", "http://127.0.0.1:11434"),
		LogLevel:    getEnvString("LOG_LEVEL", "info"),

		// Storage
		Storage:        StorageType(getEnvString("STORAGE", string(storageDefault))),
		StoragePath:    getEnvString("STORAGE_PATH", "/data/oac.sqlite"),
		StorageMaxRows: getEnvInt("STORAGE_MAX_ROWS", 3000),

		// Retry
		RetryMax:       getEnvInt("RETRY_MAX", 2),
		RetryBackoffMs: getEnvInt("RETRY_BACKOFF_MS", 1000),

		// Protect
		TimeoutTTFBMs:        getEnvInt("TIMEOUT_TTFB_MS", 15000),
		TimeoutStallMs:       getEnvInt("TIMEOUT_STALL_MS", 30000),
		TimeoutHardMs:        getEnvInt("TIMEOUT_HARD_MS", 300000),
		LoopDetectEnabled:    getEnvBool("LOOP_DETECT_ENABLED", true),
		LoopWindowBytes:      getEnvInt("LOOP_WINDOW_BYTES", 4096),
		LoopNgramBytes:       getEnvInt("LOOP_NGRAM_BYTES", 64),
		LoopRepeatThreshold:  getEnvInt("LOOP_REPEAT_THRESHOLD", 3),
		LoopMinOutputBytes:   getEnvInt("LOOP_MIN_OUTPUT_BYTES", 1024),
		OutputLimitEnabled:   getEnvBool("OUTPUT_LIMIT_ENABLED", true),
		OutputLimitMaxTokens: getEnvInt("OUTPUT_LIMIT_MAX_TOKENS", 4096),

		// Context window
		MinCtx:   getEnvInt("MIN_CTX", 1024),
		MaxCtx:   getEnvInt("MAX_CTX", 81920),
		Buckets:  getEnvIntList("BUCKETS", []int{1024, 2048, 4096, 8192, 9216, 10240, 11264, 12288, 13312, 14336, 15360, 16384, 20480, 24576, 28672, 32768, 36864, 40960, 45056, 49152, 53248, 57344, 61440, 65536, 69632, 73728, 77824, 81920, 86016, 90112, 94208, 98304, 102400}),
		Headroom: getEnvFloat("HEADROOM", 1.25),

		// Output budgeting
		DefaultOutputBudget:        getEnvInt("DEFAULT_OUTPUT_BUDGET", 1024),
		MaxOutputBudget:            getEnvInt("MAX_OUTPUT_BUDGET", 10240),
		StructuredOverhead:         getEnvInt("STRUCTURED_OVERHEAD", 128),
		DynamicDefaultOutputBudget: getEnvBool("DYNAMIC_DEFAULT_OUTPUT_BUDGET", false),

		// Estimation defaults
		DefaultFixedOverheadTokens:    getEnvFloat("DEFAULT_FIXED_OVERHEAD_TOKENS", 32),
		DefaultPerMessageOverhead:     getEnvFloat("DEFAULT_PER_MESSAGE_OVERHEAD_TOKENS", 8),
		DefaultTokensPerByte:          getEnvFloat("DEFAULT_TOKENS_PER_BYTE", 0.25),
		DefaultTokensPerImageFallback: getEnvInt("DEFAULT_TOKENS_PER_IMAGE", 768),

		OverrideNumCtx: OverridePolicy(getEnvString("OVERRIDE_NUM_CTX", string(OverrideIfTooSmall))),

		// Safety + performance
		RequestBodyMaxBytes: getEnvInt64("REQUEST_BODY_MAX_BYTES", 10*1024*1024),
		ResponseTapMaxBytes: getEnvInt64("RESPONSE_TAP_MAX_BYTES", 5*1024*1024),
		ShowCacheTTL:        getEnvDuration("SHOW_CACHE_TTL", 5*time.Minute),
		CalibrationEnabled:  getEnvBool("CALIBRATION_ENABLED", true),
		CalibrationFile:     getEnvString("CALIBRATION_FILE", ""),
		ProgressInterval:    getEnvDuration("PROGRESS_INTERVAL", 250*time.Millisecond),
		RecentBuffer:        getEnvInt("RECENT_BUFFER", 200),
		HealthCheckInterval: getEnvDuration("HEALTH_CHECK_INTERVAL", 30*time.Second),
		HealthCheckTimeout:  getEnvDuration("HEALTH_CHECK_TIMEOUT", 5*time.Second),

		// HTTP
		CORSAllowOrigin: getEnvString("CORS_ALLOW_ORIGIN", "*"),
		FlushInterval:   getEnvDuration("FLUSH_INTERVAL", 100*time.Millisecond),

		// System prompt
		StripSystemPromptText: getEnvString("STRIP_SYSTEM_PROMPT_TEXT", ""),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks configuration constraints.
func (c Config) Validate() error {
	// Mode validation
	switch c.Mode {
	case ModeOff, ModeMonitor, ModeRetry, ModeProtect:
		// ok
	default:
		return fmt.Errorf("invalid MODE: %q (must be off|monitor|retry|protect)", c.Mode)
	}

	// Storage validation
	switch c.Storage {
	case StorageSQLite, StorageMemory, StorageOff:
		// ok
	default:
		return fmt.Errorf("invalid STORAGE: %q (must be sqlite|memory|off)", c.Storage)
	}

	if c.StorageMaxRows < 100 {
		return fmt.Errorf("STORAGE_MAX_ROWS must be >= 100")
	}

	// Context validation
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

	// Output validation
	if c.DefaultOutputBudget < 0 || c.MaxOutputBudget < 0 {
		return fmt.Errorf("output budgets must be >= 0")
	}
	if c.DefaultOutputBudget > c.MaxOutputBudget {
		return fmt.Errorf("DEFAULT_OUTPUT_BUDGET must be <= MAX_OUTPUT_BUDGET")
	}

	// Retry validation
	if c.RetryMax < 1 {
		return fmt.Errorf("RETRY_MAX must be >= 1")
	}
	if c.RetryBackoffMs < 0 {
		return fmt.Errorf("RETRY_BACKOFF_MS must be >= 0")
	}

	// Protect validation
	if c.TimeoutTTFBMs <= 0 {
		return fmt.Errorf("TIMEOUT_TTFB_MS must be > 0")
	}
	if c.TimeoutStallMs <= 0 {
		return fmt.Errorf("TIMEOUT_STALL_MS must be > 0")
	}
	if c.TimeoutHardMs <= 0 {
		return fmt.Errorf("TIMEOUT_HARD_MS must be > 0")
	}
	if c.LoopWindowBytes < 256 {
		return fmt.Errorf("LOOP_WINDOW_BYTES must be >= 256")
	}
	if c.LoopNgramBytes < 8 {
		return fmt.Errorf("LOOP_NGRAM_BYTES must be >= 8")
	}
	if c.LoopRepeatThreshold < 2 {
		return fmt.Errorf("LOOP_REPEAT_THRESHOLD must be >= 2")
	}
	if c.LoopMinOutputBytes < 256 {
		return fmt.Errorf("LOOP_MIN_OUTPUT_BYTES must be >= 256")
	}
	if c.OutputLimitMaxTokens < 0 {
		return fmt.Errorf("OUTPUT_LIMIT_MAX_TOKENS must be >= 0")
	}

	// Override policy
	switch c.OverrideNumCtx {
	case OverrideAlways, OverrideIfMissing, OverrideIfTooSmall:
		// ok
	default:
		return fmt.Errorf("invalid OVERRIDE_NUM_CTX: %q", c.OverrideNumCtx)
	}

	// Buckets validation
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

	// Progress interval
	if c.ProgressInterval <= 0 {
		return fmt.Errorf("PROGRESS_INTERVAL must be > 0")
	}

	// Recent buffer
	if c.RecentBuffer < 0 {
		return fmt.Errorf("RECENT_BUFFER must be >= 0")
	}

	// Health check
	if c.HealthCheckInterval <= 0 {
		return fmt.Errorf("HEALTH_CHECK_INTERVAL must be > 0")
	}
	if c.HealthCheckTimeout <= 0 {
		return fmt.Errorf("HEALTH_CHECK_TIMEOUT must be > 0")
	}

	return nil
}

// Helper functions for parsing environment variables

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
