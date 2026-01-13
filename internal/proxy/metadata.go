package proxy

import (
	"ollama-auto-ctx/internal/storage"
)

// RequestMeta holds parsed metadata from request body.
// No content is stored - only structural metrics.
type RequestMeta struct {
	Model           string
	Endpoint        string
	MessagesCount   int
	SystemChars     int
	UserChars       int
	AssistantChars  int
	ToolsCount      int
	ToolChoice      string
	StreamRequested bool
	ClientInBytes   int64
}

// ParseRequestMetadata extracts metadata from a parsed request.
// This only captures structural information - no actual content is stored.
func ParseRequestMetadata(endpoint string, reqMap map[string]any, bodyLen int) RequestMeta {
	meta := RequestMeta{
		ClientInBytes: int64(bodyLen),
		Endpoint:      endpoint,
	}

	// Model
	if model, ok := reqMap["model"].(string); ok && model != "" {
		meta.Model = model
	} else {
		meta.Model = "unknown"
	}

	// Stream flag
	if stream, ok := reqMap["stream"].(bool); ok {
		meta.StreamRequested = stream
	}

	// Parse based on endpoint type
	switch endpoint {
	case "chat":
		parseChatMetadata(&meta, reqMap)
	case "generate":
		parseGenerateMetadata(&meta, reqMap)
	}

	return meta
}

// parseChatMetadata extracts metadata from /api/chat requests.
func parseChatMetadata(meta *RequestMeta, reqMap map[string]any) {
	// Messages array
	if msgs, ok := reqMap["messages"].([]any); ok {
		meta.MessagesCount = len(msgs)

		for _, m := range msgs {
			msg, ok := m.(map[string]any)
			if !ok {
				continue
			}

			role, _ := msg["role"].(string)
			content := extractContentLength(msg["content"])

			switch role {
			case "system":
				meta.SystemChars += content
			case "user":
				meta.UserChars += content
			case "assistant":
				meta.AssistantChars += content
			}
		}
	}

	// Tools
	if tools, ok := reqMap["tools"].([]any); ok {
		meta.ToolsCount = len(tools)
	}

	// Tool choice
	if tc, ok := reqMap["tool_choice"].(string); ok {
		meta.ToolChoice = tc
	} else if tc, ok := reqMap["tool_choice"].(map[string]any); ok {
		if t, ok := tc["type"].(string); ok {
			meta.ToolChoice = t
		}
	}
}

// parseGenerateMetadata extracts metadata from /api/generate requests.
func parseGenerateMetadata(meta *RequestMeta, reqMap map[string]any) {
	// System prompt
	if sys, ok := reqMap["system"].(string); ok {
		meta.SystemChars = len(sys)
	}

	// Main prompt
	if prompt, ok := reqMap["prompt"].(string); ok {
		meta.UserChars = len(prompt)
	}

	// Generate doesn't have messages
	meta.MessagesCount = 0
}

// extractContentLength handles both string and array content formats.
func extractContentLength(content any) int {
	switch c := content.(type) {
	case string:
		return len(c)
	case []any:
		// Multi-modal content array
		total := 0
		for _, part := range c {
			if p, ok := part.(map[string]any); ok {
				if text, ok := p["text"].(string); ok {
					total += len(text)
				}
			}
		}
		return total
	default:
		return 0
	}
}

// ToStorageRequest creates a storage.Request from metadata.
func (m *RequestMeta) ToStorageRequest(id string, tsStart int64) *storage.Request {
	return &storage.Request{
		ID:              id,
		TSStart:         tsStart,
		Status:          storage.StatusInFlight,
		Model:           m.Model,
		Endpoint:        m.Endpoint,
		MessagesCount:   m.MessagesCount,
		SystemChars:     m.SystemChars,
		UserChars:       m.UserChars,
		AssistantChars:  m.AssistantChars,
		ToolsCount:      m.ToolsCount,
		ToolChoice:      m.ToolChoice,
		StreamRequested: m.StreamRequested,
		ClientInBytes:   m.ClientInBytes,
	}
}
