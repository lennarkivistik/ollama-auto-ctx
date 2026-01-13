package proxy

import (
	"testing"
)

func TestParseRequestMetadata_Chat(t *testing.T) {
	reqMap := map[string]any{
		"model":  "llama2",
		"stream": true,
		"messages": []any{
			map[string]any{"role": "system", "content": "You are a helpful assistant."},
			map[string]any{"role": "user", "content": "Hello!"},
			map[string]any{"role": "assistant", "content": "Hi there!"},
			map[string]any{"role": "user", "content": "How are you?"},
		},
		"tools": []any{
			map[string]any{"name": "tool1"},
			map[string]any{"name": "tool2"},
		},
		"tool_choice": "auto",
	}

	meta := ParseRequestMetadata("chat", reqMap, 500)

	if meta.Model != "llama2" {
		t.Errorf("Model = %q, want %q", meta.Model, "llama2")
	}
	if meta.Endpoint != "chat" {
		t.Errorf("Endpoint = %q, want %q", meta.Endpoint, "chat")
	}
	if !meta.StreamRequested {
		t.Error("StreamRequested should be true")
	}
	if meta.MessagesCount != 4 {
		t.Errorf("MessagesCount = %d, want 4", meta.MessagesCount)
	}
	if meta.SystemChars != 28 { // "You are a helpful assistant."
		t.Errorf("SystemChars = %d, want 28", meta.SystemChars)
	}
	if meta.UserChars != 18 { // "Hello!" + "How are you?"
		t.Errorf("UserChars = %d, want 18", meta.UserChars)
	}
	if meta.AssistantChars != 9 { // "Hi there!"
		t.Errorf("AssistantChars = %d, want 9", meta.AssistantChars)
	}
	if meta.ToolsCount != 2 {
		t.Errorf("ToolsCount = %d, want 2", meta.ToolsCount)
	}
	if meta.ToolChoice != "auto" {
		t.Errorf("ToolChoice = %q, want %q", meta.ToolChoice, "auto")
	}
	if meta.ClientInBytes != 500 {
		t.Errorf("ClientInBytes = %d, want 500", meta.ClientInBytes)
	}
}

func TestParseRequestMetadata_Generate(t *testing.T) {
	reqMap := map[string]any{
		"model":  "codellama",
		"system": "You are a coding assistant.",
		"prompt": "Write a hello world function in Go.",
		"stream": false,
	}

	meta := ParseRequestMetadata("generate", reqMap, 200)

	if meta.Model != "codellama" {
		t.Errorf("Model = %q, want %q", meta.Model, "codellama")
	}
	if meta.Endpoint != "generate" {
		t.Errorf("Endpoint = %q, want %q", meta.Endpoint, "generate")
	}
	if meta.StreamRequested {
		t.Error("StreamRequested should be false")
	}
	if meta.SystemChars != 27 { // "You are a coding assistant."
		t.Errorf("SystemChars = %d, want 27", meta.SystemChars)
	}
	if meta.UserChars != 35 { // "Write a hello world function in Go."
		t.Errorf("UserChars = %d, want 35", meta.UserChars)
	}
	if meta.MessagesCount != 0 {
		t.Errorf("MessagesCount = %d, want 0", meta.MessagesCount)
	}
}

func TestParseRequestMetadata_UnknownModel(t *testing.T) {
	reqMap := map[string]any{
		"messages": []any{},
	}

	meta := ParseRequestMetadata("chat", reqMap, 100)

	if meta.Model != "unknown" {
		t.Errorf("Model = %q, want %q", meta.Model, "unknown")
	}
}

func TestParseRequestMetadata_MultiModalContent(t *testing.T) {
	reqMap := map[string]any{
		"model": "llava",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "What is in this image?"},
					map[string]any{"type": "image", "image_url": "..."},
				},
			},
		},
	}

	meta := ParseRequestMetadata("chat", reqMap, 300)

	if meta.MessagesCount != 1 {
		t.Errorf("MessagesCount = %d, want 1", meta.MessagesCount)
	}
	if meta.UserChars != 22 { // "What is in this image?"
		t.Errorf("UserChars = %d, want 22", meta.UserChars)
	}
}

func TestToStorageRequest(t *testing.T) {
	meta := RequestMeta{
		Model:           "llama2",
		Endpoint:        "chat",
		MessagesCount:   5,
		SystemChars:     100,
		UserChars:       200,
		AssistantChars:  150,
		ToolsCount:      2,
		ToolChoice:      "auto",
		StreamRequested: true,
		ClientInBytes:   500,
	}

	req := meta.ToStorageRequest("req-123", 1234567890)

	if req.ID != "req-123" {
		t.Errorf("ID = %q, want %q", req.ID, "req-123")
	}
	if req.TSStart != 1234567890 {
		t.Errorf("TSStart = %d, want 1234567890", req.TSStart)
	}
	if req.Model != "llama2" {
		t.Errorf("Model = %q, want %q", req.Model, "llama2")
	}
	if req.MessagesCount != 5 {
		t.Errorf("MessagesCount = %d, want 5", req.MessagesCount)
	}
	if !req.StreamRequested {
		t.Error("StreamRequested should be true")
	}
}
