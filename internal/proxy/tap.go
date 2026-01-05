package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"strings"

	"ollama-auto-ctx/internal/calibration"
	"ollama-auto-ctx/internal/util"
)

// TapReadCloser wraps an upstream response body and "taps" the bytes
// to extract Ollama's prompt_eval_count for auto-calibration.
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

	sample calibration.Sample
	store  *calibration.Store
	logger *slog.Logger

	// ndjsonBuf holds any incomplete line between reads.
	ndjsonBuf []byte

	// jsonBuf buffers non-stream JSON bodies up to maxBuffer.
	jsonBuf          []byte
	jsonBufTruncated bool

	observed bool
}

// NewTapReadCloser wraps rc and returns a ReadCloser that updates the calibration store.
func NewTapReadCloser(rc io.ReadCloser, contentType string, _ int64, maxBuffer int64, sample calibration.Sample, store *calibration.Store, logger *slog.Logger) io.ReadCloser {
	ctLower := strings.ToLower(contentType)
	isNDJSON := strings.Contains(ctLower, "application/x-ndjson")
	isJSON := strings.Contains(ctLower, "application/json")

	return &TapReadCloser{
		rc:        rc,
		isNDJSON:  isNDJSON,
		isJSON:    isJSON,
		maxBuffer: maxBuffer,
		sample:    sample,
		store:     store,
		logger:    logger,
	}
}

func (t *TapReadCloser) Read(p []byte) (int, error) {
	n, err := t.rc.Read(p)
	if n > 0 && !t.observed {
		t.process(p[:n])
	}
	if err == io.EOF {
		// Best-effort parse of any trailing bytes.
		if !t.observed {
			t.finish()
		}
	}
	return n, err
}

func (t *TapReadCloser) Close() error {
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
			t.tryParseJSON(line)
			if t.observed {
				return
			}
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
	if t.observed {
		return
	}
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
	v, ok := m["prompt_eval_count"]
	if !ok {
		return
	}
	n, ok := util.ToInt(v)
	if !ok || n <= 0 {
		return
	}

	t.store.Update(t.sample, calibration.Observed{PromptEvalCount: n})
	t.observed = true
	if t.logger != nil {
		t.logger.Debug("calibration observation", "model", t.sample.Model, "prompt_eval_count", n)
	}
}
