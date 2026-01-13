package storage

import (
	"sort"
	"sync"
	"time"
)

// MemoryStore implements Store using an in-memory ring buffer.
// This is used when STORAGE=memory or as a fallback.
type MemoryStore struct {
	mu       sync.RWMutex
	requests []Request
	byID     map[string]int // ID -> index in requests
	maxRows  int
	head     int // next write position
	count    int // actual count (may be less than len(requests) initially)
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore(maxRows int) *MemoryStore {
	return &MemoryStore{
		requests: make([]Request, maxRows),
		byID:     make(map[string]int),
		maxRows:  maxRows,
	}
}

// Insert adds a new request to the store.
func (s *MemoryStore) Insert(req *Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If we're overwriting, remove old ID from map
	if s.count == s.maxRows {
		oldID := s.requests[s.head].ID
		delete(s.byID, oldID)
	}

	// Store at head position
	s.requests[s.head] = *req
	s.byID[req.ID] = s.head

	// Advance head
	s.head = (s.head + 1) % s.maxRows
	if s.count < s.maxRows {
		s.count++
	}

	return nil
}

// Update modifies an existing request.
func (s *MemoryStore) Update(id string, upd RequestUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, ok := s.byID[id]
	if !ok {
		return nil // not found is not an error
	}

	req := &s.requests[idx]

	if upd.TSEnd != nil {
		req.TSEnd = upd.TSEnd
	}
	if upd.Status != nil {
		req.Status = *upd.Status
	}
	if upd.Reason != nil {
		req.Reason = *upd.Reason
	}
	if upd.CtxEst != nil {
		req.CtxEst = *upd.CtxEst
	}
	if upd.CtxSelected != nil {
		req.CtxSelected = *upd.CtxSelected
	}
	if upd.CtxBucket != nil {
		req.CtxBucket = *upd.CtxBucket
	}
	if upd.OutputBudget != nil {
		req.OutputBudget = *upd.OutputBudget
	}
	if upd.PromptTokens != nil {
		req.PromptTokens = *upd.PromptTokens
	}
	if upd.CompletionTokens != nil {
		req.CompletionTokens = *upd.CompletionTokens
	}
	if upd.DurationMs != nil {
		req.DurationMs = *upd.DurationMs
	}
	if upd.TTFBMs != nil {
		req.TTFBMs = *upd.TTFBMs
	}
	if upd.UpstreamTotalMs != nil {
		req.UpstreamTotalMs = *upd.UpstreamTotalMs
	}
	if upd.UpstreamLoadMs != nil {
		req.UpstreamLoadMs = *upd.UpstreamLoadMs
	}
	if upd.UpstreamPromptEvalMs != nil {
		req.UpstreamPromptEvalMs = *upd.UpstreamPromptEvalMs
	}
	if upd.UpstreamEvalMs != nil {
		req.UpstreamEvalMs = *upd.UpstreamEvalMs
	}
	if upd.ClientOutBytes != nil {
		req.ClientOutBytes = *upd.ClientOutBytes
	}
	if upd.UpstreamInBytes != nil {
		req.UpstreamInBytes = *upd.UpstreamInBytes
	}
	if upd.UpstreamOutBytes != nil {
		req.UpstreamOutBytes = *upd.UpstreamOutBytes
	}
	if upd.RetryCount != nil {
		req.RetryCount = *upd.RetryCount
	}
	if upd.UpstreamHTTPStatus != nil {
		req.UpstreamHTTPStatus = *upd.UpstreamHTTPStatus
	}
	if upd.ErrorClass != nil {
		req.ErrorClass = *upd.ErrorClass
	}

	return nil
}

// GetByID retrieves a single request.
func (s *MemoryStore) GetByID(id string) (*Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	idx, ok := s.byID[id]
	if !ok {
		return nil, nil
	}

	req := s.requests[idx]
	return &req, nil
}

// List returns requests matching the filter options.
func (s *MemoryStore) List(opts ListOptions) ([]Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect all requests in order (newest first)
	all := s.collectOrdered()

	// Apply filters
	var filtered []Request
	cutoff := int64(0)
	if opts.Window > 0 {
		cutoff = time.Now().UnixMilli() - opts.Window.Milliseconds()
	}

	for _, req := range all {
		if opts.Status != nil && req.Status != *opts.Status {
			continue
		}
		if opts.Model != "" && req.Model != opts.Model {
			continue
		}
		if opts.Reason != nil && req.Reason != *opts.Reason {
			continue
		}
		if cutoff > 0 && req.TSStart < cutoff {
			continue
		}
		filtered = append(filtered, req)
	}

	// Apply pagination
	if opts.Offset >= len(filtered) {
		return nil, nil
	}
	filtered = filtered[opts.Offset:]
	if opts.Limit > 0 && opts.Limit < len(filtered) {
		filtered = filtered[:opts.Limit]
	}

	return filtered, nil
}

// Overview returns aggregate statistics.
func (s *MemoryStore) Overview(window time.Duration) (*Overview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().UnixMilli() - window.Milliseconds()
	all := s.collectOrdered()

	var o Overview
	var durations []int

	for _, req := range all {
		if req.TSStart < cutoff {
			continue
		}

		o.TotalRequests++
		if req.Status == StatusSuccess {
			o.SuccessCount++
		} else if req.Status == StatusError || req.Status == StatusCanceled {
			o.ErrorCount++
		}

		if req.Status != StatusInFlight && req.DurationMs > 0 {
			durations = append(durations, req.DurationMs)
		}

		o.TotalBytes += req.ClientOutBytes
		o.TotalTokens += req.CompletionTokens
		o.Retries += req.RetryCount

		switch req.Reason {
		case ReasonTimeoutTTFB, ReasonTimeoutStall, ReasonTimeoutHard:
			o.Timeouts++
		case ReasonLoopDetected:
			o.Loops++
		}
	}

	if o.TotalRequests > 0 {
		o.SuccessRate = float64(o.SuccessCount) / float64(o.TotalRequests)
	}

	if len(durations) > 0 {
		sort.Ints(durations)
		sum := 0
		for _, d := range durations {
			sum += d
		}
		o.AvgDurationMs = sum / len(durations)

		p95Idx := int(float64(len(durations)) * 0.95)
		if p95Idx >= len(durations) {
			p95Idx = len(durations) - 1
		}
		o.P95DurationMs = durations[p95Idx]
	}

	return &o, nil
}

// ModelStats returns per-model statistics.
func (s *MemoryStore) ModelStats(window time.Duration) ([]ModelStat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoff := time.Now().UnixMilli() - window.Milliseconds()
	all := s.collectOrdered()

	// Group by model
	byModel := make(map[string][]Request)
	for _, req := range all {
		if req.TSStart < cutoff || req.Model == "" {
			continue
		}
		byModel[req.Model] = append(byModel[req.Model], req)
	}

	var stats []ModelStat
	for model, reqs := range byModel {
		ms := ModelStat{
			Model:        model,
			RequestCount: len(reqs),
		}

		var successCount, totalRetries, loadCount int
		var ctxSum int

		for _, req := range reqs {
			if req.Status == StatusSuccess {
				successCount++
			}
			totalRetries += req.RetryCount
			ctxSum += req.CtxSelected
			if req.UpstreamLoadMs > 0 {
				loadCount++
			}
		}

		if ms.RequestCount > 0 {
			ms.SuccessRate = float64(successCount) / float64(ms.RequestCount)
			ms.AvgCtxSelected = ctxSum / ms.RequestCount
			ms.RetryRate = float64(totalRetries) / float64(ms.RequestCount)
			ms.LoadChurnRate = float64(loadCount) / float64(ms.RequestCount)
		}

		stats = append(stats, ms)
	}

	// Sort by request count descending
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].RequestCount > stats[j].RequestCount
	})

	return stats, nil
}

// Series returns time-binned data for charts.
func (s *MemoryStore) Series(opts SeriesOptions) ([]DataPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bins, interval := GetBinConfig(opts.Window)
	now := time.Now()
	cutoff := now.Add(-opts.Window)

	// Initialize bins
	points := make([]DataPoint, bins)
	for i := 0; i < bins; i++ {
		ts := cutoff.Add(time.Duration(i) * interval)
		points[i] = DataPoint{Timestamp: ts.UnixMilli(), Value: 0}
	}

	binValues := make([][]float64, bins)
	for i := range binValues {
		binValues[i] = make([]float64, 0)
	}

	all := s.collectOrdered()
	for _, req := range all {
		if req.TSStart < cutoff.UnixMilli() {
			continue
		}
		if opts.Model != "" && req.Model != opts.Model {
			continue
		}

		binIdx := int((req.TSStart - cutoff.UnixMilli()) / interval.Milliseconds())
		if binIdx < 0 || binIdx >= bins {
			continue
		}

		var value float64
		switch opts.Metric {
		case "req_count":
			value = 1
		case "duration_p95":
			if req.Status != StatusInFlight {
				value = float64(req.DurationMs)
			}
		case "gen_tok_per_s":
			if req.Status == StatusSuccess && req.DurationMs > 0 {
				value = float64(req.CompletionTokens) * 1000.0 / float64(req.DurationMs)
			}
		case "ctx_utilization":
			if req.Status == StatusSuccess && req.CtxSelected > 0 {
				value = float64(req.PromptTokens+req.CompletionTokens) / float64(req.CtxSelected)
			}
		}

		binValues[binIdx] = append(binValues[binIdx], value)
	}

	// Aggregate
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
			sort.Float64s(vals)
			mid := len(vals) / 2
			if len(vals)%2 == 0 {
				points[i].Value = (vals[mid-1] + vals[mid]) / 2
			} else {
				points[i].Value = vals[mid]
			}
		}
	}

	return points, nil
}

// InFlightCount returns the number of in-flight requests.
func (s *MemoryStore) InFlightCount() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for i := 0; i < s.count; i++ {
		idx := (s.head - 1 - i + s.maxRows) % s.maxRows
		if s.requests[idx].Status == StatusInFlight {
			count++
		}
	}
	return count, nil
}

// Close is a no-op for memory store.
func (s *MemoryStore) Close() error {
	return nil
}

// collectOrdered returns all requests sorted by ts_start descending (newest first).
func (s *MemoryStore) collectOrdered() []Request {
	if s.count == 0 {
		return nil
	}

	result := make([]Request, 0, s.count)
	for i := 0; i < s.count; i++ {
		// Start from head-1 (most recent) and go backward
		idx := (s.head - 1 - i + s.maxRows) % s.maxRows
		result = append(result, s.requests[idx])
	}
	return result
}
