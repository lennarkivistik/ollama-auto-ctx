//go:build !mips64 && !mips64le && !ppc64 && !s390x

package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver (no CGO required)
)

const schema = `
CREATE TABLE IF NOT EXISTS requests (
    id TEXT PRIMARY KEY,
    ts_start INTEGER NOT NULL,
    ts_end INTEGER,
    status TEXT NOT NULL DEFAULT 'in_flight',
    reason TEXT,
    model TEXT,
    endpoint TEXT,
    
    messages_count INTEGER DEFAULT 0,
    system_chars INTEGER DEFAULT 0,
    user_chars INTEGER DEFAULT 0,
    assistant_chars INTEGER DEFAULT 0,
    tools_count INTEGER DEFAULT 0,
    tool_choice TEXT,
    stream_requested INTEGER DEFAULT 0,
    
    ctx_est INTEGER DEFAULT 0,
    ctx_selected INTEGER DEFAULT 0,
    ctx_bucket INTEGER DEFAULT 0,
    output_budget INTEGER DEFAULT 0,
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    
    duration_ms INTEGER DEFAULT 0,
    ttfb_ms INTEGER DEFAULT 0,
    upstream_total_ms INTEGER DEFAULT 0,
    upstream_load_ms INTEGER DEFAULT 0,
    upstream_prompt_eval_ms INTEGER DEFAULT 0,
    upstream_eval_ms INTEGER DEFAULT 0,
    
    client_in_bytes INTEGER DEFAULT 0,
    client_out_bytes INTEGER DEFAULT 0,
    upstream_in_bytes INTEGER DEFAULT 0,
    upstream_out_bytes INTEGER DEFAULT 0,
    
    retry_count INTEGER DEFAULT 0,
    upstream_http_status INTEGER DEFAULT 0,
    error_class TEXT
);

CREATE INDEX IF NOT EXISTS idx_requests_ts_start ON requests(ts_start);
CREATE INDEX IF NOT EXISTS idx_requests_model_ts ON requests(model, ts_start);
CREATE INDEX IF NOT EXISTS idx_requests_status_ts ON requests(status, ts_start);
`

// SQLiteStore implements Store using SQLite with WAL mode.
type SQLiteStore struct {
	db         *sql.DB
	maxRows    int
	pruneMu    sync.Mutex
	pruneOnce  bool
	logger     *slog.Logger
}

// NewSQLiteStore creates a new SQLite store at the given path.
// It enables WAL mode for better concurrent performance.
func NewSQLiteStore(path string, maxRows int, logger *slog.Logger) (*SQLiteStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create storage directory: %w", err)
		}
	}

	// Open with WAL mode and normal sync for performance
	dsn := fmt.Sprintf("%s?_journal=WAL&_sync=NORMAL&_busy_timeout=5000", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite works best with single writer
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// Create schema
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &SQLiteStore{
		db:      db,
		maxRows: maxRows,
		logger:  logger,
	}, nil
}

// Insert creates a new request record.
func (s *SQLiteStore) Insert(req *Request) error {
	_, err := s.db.Exec(`
		INSERT INTO requests (
			id, ts_start, ts_end, status, reason, model, endpoint,
			messages_count, system_chars, user_chars, assistant_chars,
			tools_count, tool_choice, stream_requested,
			ctx_est, ctx_selected, ctx_bucket, output_budget,
			prompt_tokens, completion_tokens,
			duration_ms, ttfb_ms, upstream_total_ms, upstream_load_ms,
			upstream_prompt_eval_ms, upstream_eval_ms,
			client_in_bytes, client_out_bytes, upstream_in_bytes, upstream_out_bytes,
			retry_count, upstream_http_status, error_class
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		req.ID, req.TSStart, req.TSEnd, req.Status, req.Reason, req.Model, req.Endpoint,
		req.MessagesCount, req.SystemChars, req.UserChars, req.AssistantChars,
		req.ToolsCount, req.ToolChoice, boolToInt(req.StreamRequested),
		req.CtxEst, req.CtxSelected, req.CtxBucket, req.OutputBudget,
		req.PromptTokens, req.CompletionTokens,
		req.DurationMs, req.TTFBMs, req.UpstreamTotalMs, req.UpstreamLoadMs,
		req.UpstreamPromptEvalMs, req.UpstreamEvalMs,
		req.ClientInBytes, req.ClientOutBytes, req.UpstreamInBytes, req.UpstreamOutBytes,
		req.RetryCount, req.UpstreamHTTPStatus, req.ErrorClass,
	)
	if err != nil {
		return fmt.Errorf("insert request: %w", err)
	}

	// Trigger pruning check (best effort, non-blocking)
	go s.maybePrune()

	return nil
}

// Update modifies an existing request.
func (s *SQLiteStore) Update(id string, upd RequestUpdate) error {
	// Build dynamic update query
	var sets []string
	var args []any

	if upd.TSEnd != nil {
		sets = append(sets, "ts_end = ?")
		args = append(args, *upd.TSEnd)
	}
	if upd.Status != nil {
		sets = append(sets, "status = ?")
		args = append(args, string(*upd.Status))
	}
	if upd.Reason != nil {
		sets = append(sets, "reason = ?")
		args = append(args, string(*upd.Reason))
	}
	if upd.CtxEst != nil {
		sets = append(sets, "ctx_est = ?")
		args = append(args, *upd.CtxEst)
	}
	if upd.CtxSelected != nil {
		sets = append(sets, "ctx_selected = ?")
		args = append(args, *upd.CtxSelected)
	}
	if upd.CtxBucket != nil {
		sets = append(sets, "ctx_bucket = ?")
		args = append(args, *upd.CtxBucket)
	}
	if upd.OutputBudget != nil {
		sets = append(sets, "output_budget = ?")
		args = append(args, *upd.OutputBudget)
	}
	if upd.PromptTokens != nil {
		sets = append(sets, "prompt_tokens = ?")
		args = append(args, *upd.PromptTokens)
	}
	if upd.CompletionTokens != nil {
		sets = append(sets, "completion_tokens = ?")
		args = append(args, *upd.CompletionTokens)
	}
	if upd.DurationMs != nil {
		sets = append(sets, "duration_ms = ?")
		args = append(args, *upd.DurationMs)
	}
	if upd.TTFBMs != nil {
		sets = append(sets, "ttfb_ms = ?")
		args = append(args, *upd.TTFBMs)
	}
	if upd.UpstreamTotalMs != nil {
		sets = append(sets, "upstream_total_ms = ?")
		args = append(args, *upd.UpstreamTotalMs)
	}
	if upd.UpstreamLoadMs != nil {
		sets = append(sets, "upstream_load_ms = ?")
		args = append(args, *upd.UpstreamLoadMs)
	}
	if upd.UpstreamPromptEvalMs != nil {
		sets = append(sets, "upstream_prompt_eval_ms = ?")
		args = append(args, *upd.UpstreamPromptEvalMs)
	}
	if upd.UpstreamEvalMs != nil {
		sets = append(sets, "upstream_eval_ms = ?")
		args = append(args, *upd.UpstreamEvalMs)
	}
	if upd.ClientOutBytes != nil {
		sets = append(sets, "client_out_bytes = ?")
		args = append(args, *upd.ClientOutBytes)
	}
	if upd.UpstreamInBytes != nil {
		sets = append(sets, "upstream_in_bytes = ?")
		args = append(args, *upd.UpstreamInBytes)
	}
	if upd.UpstreamOutBytes != nil {
		sets = append(sets, "upstream_out_bytes = ?")
		args = append(args, *upd.UpstreamOutBytes)
	}
	if upd.RetryCount != nil {
		sets = append(sets, "retry_count = ?")
		args = append(args, *upd.RetryCount)
	}
	if upd.UpstreamHTTPStatus != nil {
		sets = append(sets, "upstream_http_status = ?")
		args = append(args, *upd.UpstreamHTTPStatus)
	}
	if upd.ErrorClass != nil {
		sets = append(sets, "error_class = ?")
		args = append(args, *upd.ErrorClass)
	}

	if len(sets) == 0 {
		return nil // nothing to update
	}

	query := "UPDATE requests SET "
	for i, set := range sets {
		if i > 0 {
			query += ", "
		}
		query += set
	}
	query += " WHERE id = ?"
	args = append(args, id)

	_, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update request: %w", err)
	}

	return nil
}

// GetByID retrieves a single request.
func (s *SQLiteStore) GetByID(id string) (*Request, error) {
	row := s.db.QueryRow(`
		SELECT id, ts_start, ts_end, status, reason, model, endpoint,
			messages_count, system_chars, user_chars, assistant_chars,
			tools_count, tool_choice, stream_requested,
			ctx_est, ctx_selected, ctx_bucket, output_budget,
			prompt_tokens, completion_tokens,
			duration_ms, ttfb_ms, upstream_total_ms, upstream_load_ms,
			upstream_prompt_eval_ms, upstream_eval_ms,
			client_in_bytes, client_out_bytes, upstream_in_bytes, upstream_out_bytes,
			retry_count, upstream_http_status, error_class
		FROM requests WHERE id = ?
	`, id)

	req, err := scanRequest(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}
	return req, nil
}

// List retrieves requests with filtering.
func (s *SQLiteStore) List(opts ListOptions) ([]Request, error) {
	query := `
		SELECT id, ts_start, ts_end, status, reason, model, endpoint,
			messages_count, system_chars, user_chars, assistant_chars,
			tools_count, tool_choice, stream_requested,
			ctx_est, ctx_selected, ctx_bucket, output_budget,
			prompt_tokens, completion_tokens,
			duration_ms, ttfb_ms, upstream_total_ms, upstream_load_ms,
			upstream_prompt_eval_ms, upstream_eval_ms,
			client_in_bytes, client_out_bytes, upstream_in_bytes, upstream_out_bytes,
			retry_count, upstream_http_status, error_class
		FROM requests WHERE 1=1
	`
	var args []any

	if opts.Status != nil {
		query += " AND status = ?"
		args = append(args, string(*opts.Status))
	}
	if opts.Model != "" {
		query += " AND model = ?"
		args = append(args, opts.Model)
	}
	if opts.Reason != nil {
		query += " AND reason = ?"
		args = append(args, string(*opts.Reason))
	}
	if opts.Window > 0 {
		cutoff := time.Now().UnixMilli() - opts.Window.Milliseconds()
		query += " AND ts_start >= ?"
		args = append(args, cutoff)
	}

	query += " ORDER BY ts_start DESC"

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", opts.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}
	defer rows.Close()

	var requests []Request
	for rows.Next() {
		req, err := scanRequestRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan request: %w", err)
		}
		requests = append(requests, *req)
	}

	return requests, rows.Err()
}

// Overview returns aggregate statistics.
func (s *SQLiteStore) Overview(window time.Duration) (*Overview, error) {
	cutoff := time.Now().UnixMilli() - window.Milliseconds()

	row := s.db.QueryRow(`
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN status = 'error' OR status = 'canceled' THEN 1 ELSE 0 END) as error_count,
			COALESCE(AVG(CASE WHEN status != 'in_flight' THEN duration_ms END), 0) as avg_duration,
			COALESCE(SUM(client_out_bytes), 0) as total_bytes,
			COALESCE(SUM(completion_tokens), 0) as total_tokens,
			COALESCE(SUM(retry_count), 0) as retries,
			SUM(CASE WHEN reason IN ('timeout_ttfb', 'timeout_stall', 'timeout_hard') THEN 1 ELSE 0 END) as timeouts,
			SUM(CASE WHEN reason = 'loop_detected' THEN 1 ELSE 0 END) as loops
		FROM requests
		WHERE ts_start >= ?
	`, cutoff)

	var o Overview
	var avgDur float64
	err := row.Scan(&o.TotalRequests, &o.SuccessCount, &o.ErrorCount, &avgDur,
		&o.TotalBytes, &o.TotalTokens, &o.Retries, &o.Timeouts, &o.Loops)
	if err != nil {
		return nil, fmt.Errorf("overview query: %w", err)
	}

	o.AvgDurationMs = int(avgDur)
	if o.TotalRequests > 0 {
		o.SuccessRate = float64(o.SuccessCount) / float64(o.TotalRequests)
	}

	// Calculate P95 duration
	p95Row := s.db.QueryRow(`
		SELECT duration_ms FROM requests
		WHERE ts_start >= ? AND status != 'in_flight'
		ORDER BY duration_ms DESC
		LIMIT 1 OFFSET ?
	`, cutoff, int(float64(o.TotalRequests)*0.05))

	var p95 int
	if err := p95Row.Scan(&p95); err == nil {
		o.P95DurationMs = p95
	}

	return &o, nil
}

// ModelStats returns per-model statistics.
func (s *SQLiteStore) ModelStats(window time.Duration) ([]ModelStat, error) {
	cutoff := time.Now().UnixMilli() - window.Milliseconds()

	rows, err := s.db.Query(`
		SELECT 
			model,
			COUNT(*) as request_count,
			AVG(CASE WHEN status = 'success' THEN 1.0 ELSE 0.0 END) as success_rate,
			AVG(ctx_selected) as avg_ctx_selected,
			SUM(retry_count) as total_retries,
			SUM(CASE WHEN upstream_load_ms > 0 THEN 1 ELSE 0 END) as load_count
		FROM requests
		WHERE ts_start >= ? AND model != ''
		GROUP BY model
		ORDER BY request_count DESC
	`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("model stats query: %w", err)
	}
	defer rows.Close()

	var stats []ModelStat
	for rows.Next() {
		var ms ModelStat
		var totalRetries, loadCount int
		err := rows.Scan(&ms.Model, &ms.RequestCount, &ms.SuccessRate,
			&ms.AvgCtxSelected, &totalRetries, &loadCount)
		if err != nil {
			return nil, fmt.Errorf("scan model stat: %w", err)
		}
		if ms.RequestCount > 0 {
			ms.RetryRate = float64(totalRetries) / float64(ms.RequestCount)
			ms.LoadChurnRate = float64(loadCount) / float64(ms.RequestCount)
		}
		stats = append(stats, ms)
	}

	return stats, rows.Err()
}

// Series returns time-binned data for charts.
func (s *SQLiteStore) Series(opts SeriesOptions) ([]DataPoint, error) {
	bins, interval := GetBinConfig(opts.Window)
	now := time.Now()
	cutoff := now.Add(-opts.Window)

	// Generate bin boundaries
	points := make([]DataPoint, bins)
	for i := 0; i < bins; i++ {
		ts := cutoff.Add(time.Duration(i) * interval)
		points[i] = DataPoint{Timestamp: ts.UnixMilli(), Value: 0}
	}

	// Query based on metric type
	var query string
	var args []any

	baseWhere := "ts_start >= ?"
	args = append(args, cutoff.UnixMilli())

	if opts.Model != "" {
		baseWhere += " AND model = ?"
		args = append(args, opts.Model)
	}

	switch opts.Metric {
	case "req_count":
		query = fmt.Sprintf(`
			SELECT ts_start, 1 as value FROM requests WHERE %s
		`, baseWhere)
	case "duration_p95":
		query = fmt.Sprintf(`
			SELECT ts_start, duration_ms as value FROM requests 
			WHERE %s AND status != 'in_flight'
		`, baseWhere)
	case "gen_tok_per_s":
		query = fmt.Sprintf(`
			SELECT ts_start, 
				CASE WHEN duration_ms > 0 THEN completion_tokens * 1000.0 / duration_ms ELSE 0 END as value
			FROM requests WHERE %s AND status = 'success'
		`, baseWhere)
	case "ctx_utilization":
		query = fmt.Sprintf(`
			SELECT ts_start,
				CASE WHEN ctx_selected > 0 THEN (prompt_tokens + completion_tokens) * 1.0 / ctx_selected ELSE 0 END as value
			FROM requests WHERE %s AND status = 'success'
		`, baseWhere)
	default:
		return points, nil
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("series query: %w", err)
	}
	defer rows.Close()

	// Collect values per bin
	binValues := make([][]float64, bins)
	for i := range binValues {
		binValues[i] = make([]float64, 0)
	}

	for rows.Next() {
		var tsStart int64
		var value float64
		if err := rows.Scan(&tsStart, &value); err != nil {
			return nil, fmt.Errorf("scan series row: %w", err)
		}

		// Find bin index
		binIdx := int((tsStart - cutoff.UnixMilli()) / interval.Milliseconds())
		if binIdx >= 0 && binIdx < bins {
			binValues[binIdx] = append(binValues[binIdx], value)
		}
	}

	// Aggregate per bin
	for i, vals := range binValues {
		if len(vals) == 0 {
			continue
		}

		switch opts.Metric {
		case "req_count":
			points[i].Value = float64(len(vals))
		case "duration_p95":
			sort.Float64s(vals)
			idx := int(float64(len(vals)) * 0.95)
			if idx >= len(vals) {
				idx = len(vals) - 1
			}
			points[i].Value = vals[idx]
		default:
			// Median for other metrics
			sort.Float64s(vals)
			mid := len(vals) / 2
			if len(vals)%2 == 0 {
				points[i].Value = (vals[mid-1] + vals[mid]) / 2
			} else {
				points[i].Value = vals[mid]
			}
		}
	}

	return points, rows.Err()
}

// InFlightCount returns the number of in-flight requests.
func (s *SQLiteStore) InFlightCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM requests WHERE status = 'in_flight'`).Scan(&count)
	return count, err
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// maybePrune checks if pruning is needed and runs it.
func (s *SQLiteStore) maybePrune() {
	s.pruneMu.Lock()
	defer s.pruneMu.Unlock()

	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM requests`).Scan(&count); err != nil {
		s.logger.Error("prune count query failed", "err", err)
		return
	}

	if count <= s.maxRows {
		return
	}

	// Delete oldest rows in batches
	toDelete := count - s.maxRows
	const batchSize = 500
	if toDelete > batchSize {
		toDelete = batchSize
	}

	_, err := s.db.Exec(`
		DELETE FROM requests WHERE id IN (
			SELECT id FROM requests ORDER BY ts_start ASC LIMIT ?
		)
	`, toDelete)
	if err != nil {
		s.logger.Error("prune failed", "err", err)
	} else {
		s.logger.Debug("pruned old requests", "deleted", toDelete)
	}
}

// Helper functions

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRequest(row rowScanner) (*Request, error) {
	var req Request
	var tsEnd sql.NullInt64
	var reason, toolChoice, errorClass sql.NullString
	var streamInt int

	err := row.Scan(
		&req.ID, &req.TSStart, &tsEnd, &req.Status, &reason, &req.Model, &req.Endpoint,
		&req.MessagesCount, &req.SystemChars, &req.UserChars, &req.AssistantChars,
		&req.ToolsCount, &toolChoice, &streamInt,
		&req.CtxEst, &req.CtxSelected, &req.CtxBucket, &req.OutputBudget,
		&req.PromptTokens, &req.CompletionTokens,
		&req.DurationMs, &req.TTFBMs, &req.UpstreamTotalMs, &req.UpstreamLoadMs,
		&req.UpstreamPromptEvalMs, &req.UpstreamEvalMs,
		&req.ClientInBytes, &req.ClientOutBytes, &req.UpstreamInBytes, &req.UpstreamOutBytes,
		&req.RetryCount, &req.UpstreamHTTPStatus, &errorClass,
	)
	if err != nil {
		return nil, err
	}

	if tsEnd.Valid {
		req.TSEnd = &tsEnd.Int64
	}
	req.Reason = Reason(reason.String)
	req.ToolChoice = toolChoice.String
	req.ErrorClass = errorClass.String
	req.StreamRequested = streamInt != 0

	return &req, nil
}

func scanRequestRows(rows *sql.Rows) (*Request, error) {
	return scanRequest(rows)
}
