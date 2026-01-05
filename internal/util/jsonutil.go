package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// DecodeJSONMap decodes JSON into a map[string]any.
//
// We enable json.Decoder.UseNumber() so numbers are preserved as json.Number.
// This avoids lossy float conversions when users pass large integer fields.
func DecodeJSONMap(b []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	// Ensure there is no trailing non-whitespace content.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("unexpected trailing JSON content")
		}
		return nil, fmt.Errorf("unexpected trailing JSON content: %w", err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

// EncodeJSON marshals a value to JSON using the standard library.
func EncodeJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}
