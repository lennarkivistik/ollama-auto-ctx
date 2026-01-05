package calibration

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Sample captures what we knew about the prompt when we sent the request.
//
// The proxy later matches this against Ollama's prompt_eval_count (actual prompt tokens)
// to continuously refine token estimation parameters.
type Sample struct {
	Model        string    `json:"model"`
	Endpoint     string    `json:"endpoint"` // "chat" or "generate"
	TextBytes    int       `json:"text_bytes"`
	MessageCount int       `json:"message_count"`
	ImageTokens  int       `json:"image_tokens"`
	UsedCtx      int       `json:"used_ctx"`
	CreatedAt    time.Time `json:"created_at"`
}

// Observed wraps an actual prompt token count from Ollama.
type Observed struct {
	PromptEvalCount int `json:"prompt_eval_count"`
}

// Params are the tunable token estimation parameters for a given model.
//
// The estimation formula is:
//   tokens ~= FixedOverhead + PerMessageOverhead*messageCount + TokensPerByte*textBytes + imageTokens
//
// Values are learned per-model using an exponential moving average (EMA).
type Params struct {
	TokensPerByte      float64 `json:"tokens_per_byte"`
	FixedOverhead      float64 `json:"fixed_overhead"`
	PerMessageOverhead float64 `json:"per_message_overhead"`
	// SafeMaxCtx is an optional dynamic clamp (e.g. if we saw an OOM at a certain ctx).
	SafeMaxCtx int `json:"safe_max_ctx"`

	UpdatedAt time.Time `json:"updated_at"`
	Samples   int       `json:"samples"`
}

// Store holds model calibration data. It is safe for concurrent use.
type Store struct {
	mu       sync.RWMutex
	alpha    float64
	defaults Params
	models   map[string]Params
	file     string
}

// NewStore creates a calibration store.
//
// If filePath is non-empty, the store will attempt to load existing parameters
// and will persist updates back to disk.
func NewStore(alpha float64, defaults Params, filePath string) *Store {
	s := &Store{
		alpha:    alpha,
		defaults: defaults,
		models:   make(map[string]Params),
		file:     filePath,
	}
	if filePath != "" {
		_ = s.Load()
	}
	return s
}

// Get returns the current parameters for a model, falling back to defaults.
func (s *Store) Get(model string) Params {
	s.mu.RLock()
	p, ok := s.models[model]
	s.mu.RUnlock()
	if ok {
		return p
	}
	p = s.defaults
	p.UpdatedAt = time.Time{}
	p.Samples = 0
	return p
}

// Update refines model parameters using a new observation.
//
// The update is intentionally conservative; it clamps values to sane ranges
// so a single anomalous response can't ruin estimation.
func (s *Store) Update(sample Sample, obs Observed) {
	if sample.Model == "" {
		return
	}
	if obs.PromptEvalCount <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.models[sample.Model]
	if !ok {
		p = s.defaults
	}

	// Predicted tokens (current params)
	pred := p.FixedOverhead + p.PerMessageOverhead*float64(sample.MessageCount) + p.TokensPerByte*float64(sample.TextBytes) + float64(sample.ImageTokens)
	actual := float64(obs.PromptEvalCount)

	// We do sequential EMA updates for each parameter.
	// This isn't perfect statistical modeling, but it's stable and self-correcting.
	//
	// 1) Update TokensPerByte from the residual after subtracting overhead terms.
	if sample.TextBytes > 0 {
		residual := actual - float64(sample.ImageTokens) - p.FixedOverhead - p.PerMessageOverhead*float64(sample.MessageCount)
		cand := residual / float64(sample.TextBytes)
		cand = clampFloat(cand, 0.05, 1.0) // [1 token/20B, 1 token/1B]
		p.TokensPerByte = ema(p.TokensPerByte, cand, s.alpha)
	}

	// 2) Update per-message overhead (only for chat-like requests)
	if sample.MessageCount > 0 {
		residual := actual - float64(sample.ImageTokens) - p.FixedOverhead - p.TokensPerByte*float64(sample.TextBytes)
		cand := residual / float64(sample.MessageCount)
		cand = clampFloat(cand, 0, 64)
		p.PerMessageOverhead = ema(p.PerMessageOverhead, cand, s.alpha)
	}

	// 3) Update fixed overhead
	residual := actual - float64(sample.ImageTokens) - p.PerMessageOverhead*float64(sample.MessageCount) - p.TokensPerByte*float64(sample.TextBytes)
	cand := clampFloat(residual, 0, 256)
	p.FixedOverhead = ema(p.FixedOverhead, cand, s.alpha)

	p.UpdatedAt = time.Now()
	p.Samples++

	// Extra safeguard: don't let prediction drift too far from actual in one update.
	_ = pred

	s.models[sample.Model] = p

	// Persist in background-ish (still synchronous, but only when file is configured).
	if s.file != "" {
		_ = s.saveLocked()
	}
}

// RecordOOM reduces the safe max ctx for a model if we see an out-of-memory error.
// This helps prevent repeated failures.
func (s *Store) RecordOOM(model string, usedCtx int) {
	if model == "" || usedCtx <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.models[model]
	if !ok {
		p = s.defaults
	}
	if p.SafeMaxCtx == 0 || usedCtx < p.SafeMaxCtx {
		p.SafeMaxCtx = usedCtx
		p.UpdatedAt = time.Now()
		s.models[model] = p
		if s.file != "" {
			_ = s.saveLocked()
		}
	}
}

// Load reads calibration parameters from disk.
func (s *Store) Load() error {
	if s.file == "" {
		return nil
	}
	b, err := os.ReadFile(s.file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var data map[string]Params
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range data {
		// Fill any zero-values with defaults (useful across version upgrades).
		if v.TokensPerByte <= 0 {
			v.TokensPerByte = s.defaults.TokensPerByte
		}
		if v.FixedOverhead <= 0 {
			v.FixedOverhead = s.defaults.FixedOverhead
		}
		if v.PerMessageOverhead <= 0 {
			v.PerMessageOverhead = s.defaults.PerMessageOverhead
		}
		s.models[k] = v
	}
	return nil
}

// saveLocked persists calibration data. Caller must hold s.mu.
func (s *Store) saveLocked() error {
	if s.file == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.file), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.models, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.file, b, 0o644)
}

func ema(old, new, alpha float64) float64 {
	return old*(1-alpha) + new*alpha
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
