package estimate

import (
	"encoding/json"
	"math"
	"strings"

	"ollama-auto-ctx/internal/calibration"
	"ollama-auto-ctx/internal/util"
)

// Endpoint names we care about. These match Ollama's endpoints we rewrite.
const (
	EndpointChat     = "chat"
	EndpointGenerate = "generate"
)

// Features summarizes the parts of a request that affect context length.
//
// We intentionally keep it independent of any specific Ollama request struct,
// because we proxy unknown fields and want this middleware to survive API changes.
type Features struct {
	Model string

	Endpoint     string
	TextBytes    int
	MessageCount int
	ImageCount   int
	Structured   bool
	Raw          bool

	// User-provided options.
	ProvidedNumCtx   int
	ProvidedNumCtxOK bool
	NumPredict       int
	NumPredictOK     bool
}

// extractThinkingFromText looks for __think=<verdict> in text and returns the verdict and cleaned text.
func extractThinkingFromText(text string) (verdict string, cleanedText string) {
	// Look for __think=<verdict> pattern
	idx := strings.Index(text, "__think=")
	if idx == -1 {
		return "", text
	}

	// Extract everything after __think=
	afterThink := text[idx+8:] // 8 = len("__think=")

	// Find the end of the verdict (space, newline, or end of string)
	endIdx := len(afterThink)
	for i, ch := range afterThink {
		if ch == ' ' || ch == '\n' || ch == '\r' || ch == '\t' {
			endIdx = i
			break
		}
	}

	verdict = afterThink[:endIdx]

	// Remove __think=<verdict> from the text
	cleanedText = text[:idx] + afterThink[endIdx:]

	// Clean up extra whitespace
	cleanedText = strings.TrimSpace(cleanedText)
	// Normalize multiple newlines
	for strings.Contains(cleanedText, "\n\n\n") {
		cleanedText = strings.ReplaceAll(cleanedText, "\n\n\n", "\n\n")
	}

	return verdict, cleanedText
}

// ExtractThinkingFromSystemPrompt extracts __think=<verdict> from system prompts.
// Returns the extracted verdict (empty if none found) and modifies reqMap to remove the directive.
func ExtractThinkingFromSystemPrompt(reqMap map[string]any, endpoint string) string {
	var extractedVerdict string

	switch endpoint {
	case EndpointGenerate:
		// Extract from "system" field
		if system, ok := util.ToString(reqMap["system"]); ok {
			verdict, cleaned := extractThinkingFromText(system)
			if verdict != "" {
				extractedVerdict = verdict
				reqMap["system"] = cleaned
			}
		}

	case EndpointChat:
		// Extract from messages with role=system
		if msgs, ok := reqMap["messages"].([]any); ok {
			for _, m := range msgs {
				mm, ok := m.(map[string]any)
				if !ok {
					continue
				}
				if role, ok := util.ToString(mm["role"]); ok && role == "system" {
					if content, ok := util.ToString(mm["content"]); ok {
						verdict, cleaned := extractThinkingFromText(content)
						if verdict != "" {
							extractedVerdict = verdict
							mm["content"] = cleaned
						}
					}
				}
			}
		}
	}

	return extractedVerdict
}

// StripSystemPromptText removes specified text from system prompts.
// It handles both /api/generate (system field) and /api/chat (messages with role=system).
func StripSystemPromptText(reqMap map[string]any, endpoint, textToStrip string) {
	if textToStrip == "" {
		return
	}

	switch endpoint {
	case EndpointGenerate:
		// Strip from "system" field
		if system, ok := util.ToString(reqMap["system"]); ok {
			cleaned := strings.ReplaceAll(system, textToStrip, "")
			// Clean up extra whitespace
			cleaned = strings.TrimSpace(cleaned)
			// Normalize multiple newlines to single newline
			cleaned = strings.ReplaceAll(cleaned, "\n\n\n", "\n\n")
			reqMap["system"] = cleaned
		}

	case EndpointChat:
		// Strip from messages with role=system
		if msgs, ok := reqMap["messages"].([]any); ok {
			for _, m := range msgs {
				mm, ok := m.(map[string]any)
				if !ok {
					continue
				}
				if role, ok := util.ToString(mm["role"]); ok && role == "system" {
					if content, ok := util.ToString(mm["content"]); ok {
						cleaned := strings.ReplaceAll(content, textToStrip, "")
						// Clean up extra whitespace
						cleaned = strings.TrimSpace(cleaned)
						// Normalize multiple newlines to single newline
						cleaned = strings.ReplaceAll(cleaned, "\n\n\n", "\n\n")
						mm["content"] = cleaned
					}
				}
			}
		}
	}
}

// ExtractFeatures computes token-relevant features from an Ollama request payload.
//
// Supported inputs:
// - /api/generate: fields like model, prompt, system, suffix, raw, images
// - /api/chat: fields like model, messages, tools, format
//
// Returns ok=false if the request doesn't contain a model name.
func ExtractFeatures(endpoint string, req map[string]any) (Features, error) {
	var f Features
	f.Endpoint = endpoint

	if model, ok := util.ToString(req["model"]); ok {
		f.Model = model
	} else {
		return Features{}, nil
	}

	// options
	if opt, ok := req["options"].(map[string]any); ok {
		if n, ok := util.ToInt(opt["num_ctx"]); ok {
			f.ProvidedNumCtx = n
			f.ProvidedNumCtxOK = true
		}
		if n, ok := util.ToInt(opt["num_predict"]); ok {
			f.NumPredict = n
			f.NumPredictOK = true
		}
	}

	// format (structured output tends to need more context headroom)
	switch v := req["format"].(type) {
	case string:
		if v == "json" {
			f.Structured = true
		}
	case map[string]any:
		// JSON schema object
		f.Structured = true
	case []any:
		// Uncommon, but treat as structured.
		f.Structured = true
	default:
		_ = v
	}

	switch endpoint {
	case EndpointGenerate:
		f = extractGenerate(f, req)
	case EndpointChat:
		f = extractChat(f, req)
	default:
		// Unknown; no-op.
	}

	return f, nil
}

func extractGenerate(f Features, req map[string]any) Features {
	if s, ok := util.ToString(req["prompt"]); ok {
		f.TextBytes += len(s)
	}
	if s, ok := util.ToString(req["system"]); ok {
		f.TextBytes += len(s)
	}
	if s, ok := util.ToString(req["suffix"]); ok {
		f.TextBytes += len(s)
	}
	if s, ok := util.ToString(req["template"]); ok {
		f.TextBytes += len(s)
	}
	if b, ok := util.ToBool(req["raw"]); ok {
		f.Raw = b
	}
	// images may be a top-level array of base64 strings
	if imgs, ok := req["images"].([]any); ok {
		f.ImageCount += len(imgs)
	}
	return f
}

func extractChat(f Features, req map[string]any) Features {
	// messages is an array of {role, content, images?, tool_calls?}
	if msgs, ok := req["messages"].([]any); ok {
		for _, m := range msgs {
			mm, ok := m.(map[string]any)
			if !ok {
				continue
			}
			f.MessageCount++
			if s, ok := util.ToString(mm["content"]); ok {
				f.TextBytes += len(s)
			}
			// Some clients include tool_calls in the message.
			if tc, ok := mm["tool_calls"]; ok {
				if b, err := json.Marshal(tc); err == nil {
					f.TextBytes += len(b)
				}
			}
			if imgs, ok := mm["images"].([]any); ok {
				f.ImageCount += len(imgs)
			}
		}
	}

	// tools is typically a list of tool definitions.
	if tools, ok := req["tools"]; ok {
		if b, err := json.Marshal(tools); err == nil {
			f.TextBytes += len(b)
		}
	}

	return f
}

// EstimatePromptTokens estimates how many tokens the prompt will consume.
//
// It uses per-model calibration parameters (TokensPerByte, overhead) and includes image tokens.
func EstimatePromptTokens(f Features, params calibration.Params, tokensPerImage int) int {
	imageTokens := 0
	if f.ImageCount > 0 {
		if tokensPerImage <= 0 {
			tokensPerImage = 0
		}
		imageTokens = tokensPerImage * f.ImageCount
	}

	est := params.FixedOverhead + params.PerMessageOverhead*float64(f.MessageCount) + params.TokensPerByte*float64(f.TextBytes) + float64(imageTokens)
	if est < 0 {
		est = 0
	}
	return int(math.Ceil(est))
}

// OutputBudgetResult contains the calculated output budget and its source.
type OutputBudgetResult struct {
	Budget int
	Source string // "explicit_num_predict", "dynamic_default", or "fixed_default"
}

// BudgetOutputTokens chooses how many tokens we should reserve for generation.
//
// If options.num_predict is present, it always wins (clamped to maxBudget).
// Otherwise, if dynamicDefault is true, computes a dynamic default based on promptTokens.
// Otherwise, uses the fixed defaultBudget.
func BudgetOutputTokens(f Features, defaultBudget, maxBudget, structuredOverhead int, dynamicDefault bool, promptTokens int) OutputBudgetResult {
	var budget int
	var source string

	// options.num_predict always wins
	if f.NumPredictOK {
		budget = f.NumPredict
		source = "explicit_num_predict"
	} else if dynamicDefault {
		// Dynamic default: max(DEFAULT_OUTPUT_BUDGET, 256 + promptTokens/2)
		dynamicDefault := int(math.Max(float64(defaultBudget), float64(256+promptTokens/2)))
		budget = dynamicDefault
		source = "dynamic_default"
	} else {
		// Fixed default
		budget = defaultBudget
		source = "fixed_default"
	}

	// Clamp to valid range
	if budget < 0 {
		budget = 0
	}
	if budget > maxBudget {
		budget = maxBudget
	}

	// Add structured overhead if format is JSON
	if f.Structured {
		budget += structuredOverhead
		// Optional: add extra bump for JSON when num_predict is not explicitly set
		if !f.NumPredictOK {
			budget += 256
			// Re-clamp after adding JSON bump
			if budget > maxBudget {
				budget = maxBudget
			}
		}
	}

	return OutputBudgetResult{Budget: budget, Source: source}
}

// ApplyHeadroom inflates needed tokens by a safety factor.
func ApplyHeadroom(neededTokens int, headroom float64) int {
	if neededTokens <= 0 {
		return 0
	}
	if headroom < 1.0 {
		headroom = 1.0
	}
	return int(math.Ceil(float64(neededTokens) * headroom))
}

// Bucketize picks the smallest bucket that is >= neededTokens.
// If no bucket is big enough, it returns neededTokens.
func Bucketize(neededTokens int, buckets []int) int {
	for _, b := range buckets {
		if b >= neededTokens {
			return b
		}
	}
	return neededTokens
}

// ClampCtx clamps ctx to [min,max]. max==0 means no upper bound.
func ClampCtx(ctx, min, max int) int {
	if ctx < min {
		ctx = min
	}
	if max > 0 && ctx > max {
		ctx = max
	}
	return ctx
}
