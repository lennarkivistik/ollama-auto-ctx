package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"strings"

	"ollama-auto-ctx/internal/calibration"
	"ollama-auto-ctx/internal/storage"
	"ollama-auto-ctx/internal/supervisor"
	"ollama-auto-ctx/internal/util"
)

// TapReadCloser wraps an upstream response body and "taps" the bytes
// to extract Ollama's prompt_eval_count for auto-calibration and timing data.
//
// This wrapper is designed to be:
// - streaming safe (it does not buffer full NDJSON streams)
// - low overhead (it stops parsing once an observation is recorded)
// - future proof (it ignores unknown fields)
//
// Note: streaming in Ollama is typically newline-delimited JSON (NDJSON), not WebSockets.
type TapReadCloser struct {
	rc io.ReadCloser

	isNDJSON bool
	isJSON   bool

	maxBuffer int64

	sample       calibration.Sample
	calibStore   *calibration.Store
	dataStore    storage.Store
	tracker      *supervisor.Tracker
	loopDetector *supervisor.LoopDetector
	requestID    string
	logger       *slog.Logger

	// Output token limiting
	outputTokenLimit  int64
	outputLimitAction string // "cancel" or "warn"
	cancelFunc        func()
	limitExceeded     bool
	minOutputBytes    int64 // minimum bytes before checking limit

	// ndjsonBuf holds any incomplete line between reads.
	ndjsonBuf []byte

	// jsonBuf buffers non-stream JSON bodies up to maxBuffer.
	jsonBuf          []byte
	jsonBufTruncated bool

	observed      bool
	firstByteSent bool
	totalBytes    int64 // total bytes read for limit checking

	// Parsed Ollama response data
	promptEvalCount      int
	evalCount            int
	loadDurationNs       int64
	promptEvalDurationNs int64
	evalDurationNs       int64
	totalDurationNs      int64
}

// NewTapReadCloser wraps rc and returns a ReadCloser that updates the calibration store.
func NewTapReadCloser(rc io.ReadCloser, contentType string, _ int64, maxBuffer int64, sample calibration.Sample, calibStore *calibration.Store, tracker *supervisor.Tracker, loopDetector *supervisor.LoopDetector, requestID string, logger *slog.Logger, outputTokenLimit int64, outputLimitAction string, cancelFunc func(), minOutputBytes int64, dataStore storage.Store) io.ReadCloser {
	ctLower := strings.ToLower(contentType)
	isNDJSON := strings.Contains(ctLower, "application/x-ndjson")
	isJSON := strings.Contains(ctLower, "application/json")

	return &TapReadCloser{
		rc:                rc,
		isNDJSON:          isNDJSON,
		isJSON:            isJSON,
		maxBuffer:         maxBuffer,
		sample:            sample,
		calibStore:        calibStore,
		dataStore:         dataStore,
		tracker:           tracker,
		loopDetector:      loopDetector,
		requestID:         requestID,
		logger:            logger,
		outputTokenLimit:  outputTokenLimit,
		outputLimitAction: outputLimitAction,
		cancelFunc:        cancelFunc,
		minOutputBytes:    minOutputBytes,
	}
}

func (t *TapReadCloser) Read(p []byte) (int, error) {
	n, err := t.rc.Read(p)
	if n > 0 {
		// Track first byte sent
		if !t.firstByteSent && t.tracker != nil && t.requestID != "" {
			t.tracker.MarkFirstByte(t.requestID)
			t.firstByteSent = true
		}

		// Track progress
		if t.tracker != nil && t.requestID != "" {
			t.tracker.MarkProgress(t.requestID, int64(n))
		}

		// Update total bytes for limit checking
		t.totalBytes += int64(n)

		// Check output token limit (only after minimum output threshold)
		if t.outputTokenLimit > 0 && !t.limitExceeded && t.totalBytes >= t.minOutputBytes {
			estimated := t.estimateOutputTokens()
			if estimated > t.outputTokenLimit {
				t.limitExceeded = true
				if t.outputLimitAction == "cancel" && t.cancelFunc != nil {
					// Cancel the request
					t.cancelFunc()
					if t.tracker != nil && t.requestID != "" {
						t.tracker.Finish(t.requestID, supervisor.StatusOutputLimitExceeded, nil)
					}
					// Return error to signal cancellation
					return n, io.EOF
				} else {
					// Warn only - log and mark in tracker but continue
					if t.logger != nil {
						t.logger.Warn("output token limit exceeded",
							"request_id", t.requestID,
							"estimated_tokens", estimated,
							"limit", t.outputTokenLimit,
							"action", "warn")
					}
					// Mark in tracker so metric is recorded when request finishes
					if t.tracker != nil && t.requestID != "" {
						t.tracker.MarkOutputLimitExceeded(t.requestID)
					}
				}
			}
		}

		// Always process chunks to capture timing data, not just for calibration
		t.process(p[:n])
	}
	if err == io.EOF {
		// Best-effort parse of any trailing bytes
		t.finish()
		// Update storage with parsed data
		t.updateStorage()
	}
	return n, err
}

// feedLoopDetector extracts text content from NDJSON and feeds it to the loop detector.
// It returns true if a loop was detected (and the request was cancelled).
func (t *TapReadCloser) feedLoopDetector(line []byte) bool {
	if t.loopDetector == nil {
		return false
	}

	// Try to extract text delta from the NDJSON line
	// Ollama returns either "response" (for /api/generate) or "message.content" (for /api/chat)
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		// If parsing fails, fail open - don't feed garbage to detector
		return false
	}

	var textDelta string

	// Check for /api/generate format: {"response": "text..."}
	if resp, ok := m["response"].(string); ok {
		textDelta = resp
	}

	// Check for /api/chat format: {"message": {"content": "text..."}}
	if msg, ok := m["message"].(map[string]any); ok {
		if content, ok := msg["content"].(string); ok {
			textDelta = content
		}
	}

	if textDelta != "" {
		return t.loopDetector.Feed([]byte(textDelta))
	}

	return false
}

func (t *TapReadCloser) Close() error {
	// Ensure we parse any remaining data and update storage
	t.finish()
	t.updateStorage()
	if t.logger != nil && t.requestID != "" {
		t.logger.Debug("TapReadCloser closed, storage updated", "id", t.requestID,
			"prompt_tokens", t.promptEvalCount, "completion_tokens", t.evalCount,
			"load_ms", t.loadDurationNs/1_000_000, "eval_ms", t.evalDurationNs/1_000_000)
	}
	return t.rc.Close()
}

func (t *TapReadCloser) process(chunk []byte) {
	if t.isNDJSON {
		t.ndjsonBuf = append(t.ndjsonBuf, chunk...)
		for {
			idx := bytes.IndexByte(t.ndjsonBuf, '\n')
			if idx < 0 {
				// Safety: cap buffer growth if something goes wrong.
				if t.maxBuffer > 0 && int64(len(t.ndjsonBuf)) > t.maxBuffer {
					t.ndjsonBuf = t.ndjsonBuf[len(t.ndjsonBuf)-int(t.maxBuffer):]
				}
				return
			}
			line := t.ndjsonBuf[:idx]
			t.ndjsonBuf = t.ndjsonBuf[idx+1:]

			line = bytes.TrimSpace(bytes.TrimSuffix(line, []byte{'\r'}))
			if len(line) == 0 {
				continue
			}

			// Feed to loop detector (fail-open: if detection fails, continue normally)
			// Loop detector will cancel the request if loop is detected
			t.feedLoopDetector(line)

			// Always parse to get timing data, not just for calibration
			t.tryParseJSON(line)
		}
	}

	if t.isJSON {
		// Buffer non-stream JSON bodies (up to maxBuffer).
		if t.maxBuffer <= 0 || t.jsonBufTruncated {
			return
		}
		remaining := t.maxBuffer - int64(len(t.jsonBuf))
		if remaining <= 0 {
			t.jsonBufTruncated = true
			return
		}
		if int64(len(chunk)) <= remaining {
			t.jsonBuf = append(t.jsonBuf, chunk...)
		} else {
			t.jsonBuf = append(t.jsonBuf, chunk[:remaining]...)
			t.jsonBufTruncated = true
		}
	}
}

func (t *TapReadCloser) finish() {
	// Parse any remaining data regardless of observation status
	if t.isNDJSON {
		// Try parsing any trailing partial line.
		line := bytes.TrimSpace(bytes.TrimSuffix(t.ndjsonBuf, []byte{'\r'}))
		if len(line) > 0 {
			t.tryParseJSON(line)
		}
		return
	}
	if t.isJSON && !t.jsonBufTruncated && len(t.jsonBuf) > 0 {
		line := bytes.TrimSpace(t.jsonBuf)
		t.tryParseJSON(line)
	}
}

func (t *TapReadCloser) tryParseJSON(line []byte) {
	// Parse into a map for forward compatibility.
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return
	}

	// Extract prompt_eval_count (input tokens)
	if v, ok := m["prompt_eval_count"]; ok {
		if n, ok := util.ToInt(v); ok && n > 0 {
			t.promptEvalCount = n
			if t.calibStore != nil && t.sample.Model != "" {
				t.calibStore.Update(t.sample, calibration.Observed{PromptEvalCount: n})
			}
			t.observed = true
			if t.logger != nil {
				t.logger.Debug("calibration observation", "model", t.sample.Model, "prompt_eval_count", n)
			}
		}
	}

	// Extract eval_count (output tokens)
	if v, ok := m["eval_count"]; ok {
		if n, ok := util.ToInt(v); ok && n > 0 {
			t.evalCount = n
		}
	}

	// Extract timing data from Ollama response (in nanoseconds)
	if v, ok := m["total_duration"]; ok {
		if n, ok := util.ToInt64(v); ok && n > 0 {
			t.totalDurationNs = n
		}
	}
	if v, ok := m["load_duration"]; ok {
		if n, ok := util.ToInt64(v); ok && n > 0 {
			t.loadDurationNs = n
		}
	}
	if v, ok := m["prompt_eval_duration"]; ok {
		if n, ok := util.ToInt64(v); ok && n > 0 {
			t.promptEvalDurationNs = n
		}
	}
	if v, ok := m["eval_duration"]; ok {
		if n, ok := util.ToInt64(v); ok && n > 0 {
			t.evalDurationNs = n
		}
	}

	// Update tracker with token counts if available
	if (t.promptEvalCount > 0 || t.evalCount > 0) && t.tracker != nil && t.requestID != "" {
		t.tracker.UpdateTokenCounts(t.requestID, t.promptEvalCount, t.evalCount)
	}
}

// updateStorage updates the storage with parsed Ollama response data.
func (t *TapReadCloser) updateStorage() {
	if t.dataStore == nil || t.requestID == "" {
		return
	}

	upd := storage.RequestUpdate{}
	hasUpdate := false

	// Token counts
	if t.promptEvalCount > 0 {
		upd.PromptTokens = &t.promptEvalCount
		hasUpdate = true
	}
	if t.evalCount > 0 {
		upd.CompletionTokens = &t.evalCount
		hasUpdate = true
	}

	// Timing data (convert nanoseconds to milliseconds)
	if t.loadDurationNs > 0 {
		loadMs := int(t.loadDurationNs / 1_000_000)
		upd.UpstreamLoadMs = &loadMs
		hasUpdate = true
	}
	if t.promptEvalDurationNs > 0 {
		promptEvalMs := int(t.promptEvalDurationNs / 1_000_000)
		upd.UpstreamPromptEvalMs = &promptEvalMs
		hasUpdate = true
	}
	if t.evalDurationNs > 0 {
		evalMs := int(t.evalDurationNs / 1_000_000)
		upd.UpstreamEvalMs = &evalMs
		hasUpdate = true
	}

	// Bytes transferred (upstream out = bytes we received from upstream)
	if t.totalBytes > 0 {
		upd.UpstreamOutBytes = &t.totalBytes
		hasUpdate = true
	}

	if hasUpdate {
		if err := t.dataStore.Update(t.requestID, upd); err != nil {
			if t.logger != nil {
				t.logger.Error("failed to update storage from tap", "err", err, "id", t.requestID)
			}
		}
	}
}

// estimateOutputTokens estimates output tokens from bytes using calibration store.
func (t *TapReadCloser) estimateOutputTokens() int64 {
	// Use calibration store if available, otherwise use default
	defaultTokensPerByte := 0.25
	if t.calibStore != nil {
		params := t.calibStore.Get(t.sample.Model)
		if params.TokensPerByte > 0 {
			defaultTokensPerByte = params.TokensPerByte
		}
	}
	return supervisor.EstimateOutputTokens(t.totalBytes, t.sample.Model, t.calibStore, defaultTokensPerByte)
}
