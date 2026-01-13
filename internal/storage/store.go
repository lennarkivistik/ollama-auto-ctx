// Package storage provides persistence for request telemetry.
// It stores metadata only - no prompt/response content.
package storage

import (
	"time"
)

// Status represents the final status of a request.
type Status string

const (
	StatusInFlight Status = "in_flight"
	StatusSuccess  Status = "success"
	StatusError    Status = "error"
	StatusCanceled Status = "canceled"
)

// Reason provides more detail about error/canceled status.
type Reason string

const (
	ReasonNone              Reason = ""
	ReasonTimeoutTTFB       Reason = "timeout_ttfb"
	ReasonTimeoutStall      Reason = "timeout_stall"
	ReasonTimeoutHard       Reason = "timeout_hard"
	ReasonUpstreamError     Reason = "upstream_error"
	ReasonLoopDetected      Reason = "loop_detected"
	ReasonOutputLimitExceeded Reason = "output_limit_exceeded"
)

// Request represents a single request's telemetry data.
// No prompt/response content is stored - only metadata.
type Request struct {
	ID       string `json:"id"`
	TSStart  int64  `json:"ts_start"`  // unix ms
	TSEnd    *int64 `json:"ts_end"`    // nullable until complete
	Status   Status `json:"status"`
	Reason   Reason `json:"reason,omitempty"`
	Model    string `json:"model"`
	Endpoint string `json:"endpoint"` // chat|generate

	// Request shape (metadata only)
	MessagesCount   int    `json:"messages_count"`
	SystemChars     int    `json:"system_chars"`
	UserChars       int    `json:"user_chars"`
	AssistantChars  int    `json:"assistant_chars"`
	ToolsCount      int    `json:"tools_count"`
	ToolChoice      string `json:"tool_choice,omitempty"`
	StreamRequested bool   `json:"stream_requested"`

	// Context + tokens
	CtxEst           int `json:"ctx_est"`
	CtxSelected      int `json:"ctx_selected"`
	CtxBucket        int `json:"ctx_bucket"`
	OutputBudget     int `json:"output_budget"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`

	// Timings (ms)
	DurationMs           int `json:"duration_ms"`
	TTFBMs               int `json:"ttfb_ms"`
	UpstreamTotalMs      int `json:"upstream_total_ms"`
	UpstreamLoadMs       int `json:"upstream_load_ms"`
	UpstreamPromptEvalMs int `json:"upstream_prompt_eval_ms"`
	UpstreamEvalMs       int `json:"upstream_eval_ms"`

	// Bytes
	ClientInBytes    int64 `json:"client_in_bytes"`
	ClientOutBytes   int64 `json:"client_out_bytes"`
	UpstreamInBytes  int64 `json:"upstream_in_bytes"`
	UpstreamOutBytes int64 `json:"upstream_out_bytes"`

	// Reliability
	RetryCount         int    `json:"retry_count"`
	UpstreamHTTPStatus int    `json:"upstream_http_status"`
	ErrorClass         string `json:"error_class,omitempty"`
}

// RequestUpdate contains fields that can be updated after insert.
type RequestUpdate struct {
	TSEnd                *int64
	Status               *Status
	Reason               *Reason
	CtxEst               *int
	CtxSelected          *int
	CtxBucket            *int
	OutputBudget         *int
	PromptTokens         *int
	CompletionTokens     *int
	DurationMs           *int
	TTFBMs               *int
	UpstreamTotalMs      *int
	UpstreamLoadMs       *int
	UpstreamPromptEvalMs *int
	UpstreamEvalMs       *int
	ClientOutBytes       *int64
	UpstreamInBytes      *int64
	UpstreamOutBytes     *int64
	RetryCount           *int
	UpstreamHTTPStatus   *int
	ErrorClass           *string
}

// ListOptions filters for listing requests.
type ListOptions struct {
	Limit  int
	Offset int
	Status *Status
	Model  string
	Reason *Reason
	Window time.Duration // only requests within this window
}

// Overview contains summary statistics for a time window.
type Overview struct {
	TotalRequests int     `json:"total_requests"`
	SuccessCount  int     `json:"success_count"`
	ErrorCount    int     `json:"error_count"`
	SuccessRate   float64 `json:"success_rate"`
	AvgDurationMs int     `json:"avg_duration_ms"`
	P95DurationMs int     `json:"p95_duration_ms"`
	TotalBytes    int64   `json:"total_bytes"`
	TotalTokens   int     `json:"total_tokens"`
	Retries       int     `json:"retries"`
	Timeouts      int     `json:"timeouts"`
	Loops         int     `json:"loops"`
}

// ModelStat contains per-model rollup statistics.
type ModelStat struct {
	Model              string  `json:"model"`
	RequestCount       int     `json:"request_count"`
	SuccessRate        float64 `json:"success_rate"`
	DurationP95Ms      int     `json:"duration_p95_ms"`
	GenTokPerSMedian   float64 `json:"gen_tok_per_s_median"`
	PromptTokPerSMedian float64 `json:"prompt_tok_per_s_median"`
	AvgCtxSelected     int     `json:"avg_ctx_selected"`
	UtilizationMedian  float64 `json:"utilization_median"`
	RetryRate          float64 `json:"retry_rate"`
	LoadChurnRate      float64 `json:"load_churn_rate"`
}

// DataPoint represents a single point in a time series.
type DataPoint struct {
	Timestamp int64   `json:"ts"`    // unix ms (bin start)
	Value     float64 `json:"value"`
}

// SeriesOptions configures time series queries.
type SeriesOptions struct {
	Window time.Duration
	Metric string // duration_p95, req_count, gen_tok_per_s, ctx_utilization
	Model  string // optional filter
}

// Store is the interface for request telemetry storage.
type Store interface {
	// Insert creates a new request record (at request start).
	Insert(req *Request) error

	// Update modifies an existing request (at completion).
	Update(id string, upd RequestUpdate) error

	// GetByID retrieves a single request by ID.
	GetByID(id string) (*Request, error)

	// List retrieves requests with filtering and pagination.
	List(opts ListOptions) ([]Request, error)

	// Overview returns aggregate statistics for a time window.
	Overview(window time.Duration) (*Overview, error)

	// ModelStats returns per-model rollup statistics.
	ModelStats(window time.Duration) ([]ModelStat, error)

	// Series returns time-binned data for charts.
	Series(opts SeriesOptions) ([]DataPoint, error)

	// InFlightCount returns the number of in-flight requests.
	InFlightCount() (int, error)

	// Close releases resources.
	Close() error
}

// GetBinConfig returns the number of bins and interval for a time window.
// Used for consistent chart layouts.
func GetBinConfig(window time.Duration) (bins int, interval time.Duration) {
	switch {
	case window <= time.Hour:
		return 60, time.Minute
	case window <= 24*time.Hour:
		return 96, 15 * time.Minute
	default:
		return 168, time.Hour
	}
}
