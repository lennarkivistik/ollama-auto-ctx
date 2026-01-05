package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ollama-auto-ctx/internal/util"
)

// Client is a minimal Ollama API client used for model introspection.
//
// The proxy only needs /api/show (for model limits and template metadata).
type Client struct {
	BaseURL *url.URL
	HTTP    *http.Client
}

// NewClient constructs an Ollama client.
func NewClient(base string) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse upstream url: %w", err)
	}
	c := &Client{
		BaseURL: u,
		HTTP: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	return c, nil
}

// ShowResponse is the response from POST /api/show.
//
// We keep fields as they appear in Ollama so future additions are ignored safely.
type ShowResponse struct {
	Modelfile   string         `json:"modelfile"`
	Parameters  string         `json:"parameters"`
	Template    string         `json:"template"`
	ModelInfo   map[string]any `json:"model_info"`
	Details     map[string]any `json:"details"`
	License     any            `json:"license"`
	Messages    any            `json:"messages"`
	Name        string         `json:"name"`
	ModifiedAt  string         `json:"modified_at"`
	Digest      string         `json:"digest"`
	Size        int64          `json:"size"`
}

// Show fetches model metadata from Ollama.
func (c *Client) Show(ctx context.Context, model string, verbose bool) (ShowResponse, error) {
	payload := map[string]any{"model": model}
	if verbose {
		payload["verbose"] = true
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return ShowResponse{}, err
	}

	u := c.BaseURL.ResolveReference(&url.URL{Path: "/api/show"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(b))
	if err != nil {
		return ShowResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return ShowResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf, _ := ioReadAllLimit(resp.Body, 1024*1024)
		return ShowResponse{}, fmt.Errorf("/api/show status %d: %s", resp.StatusCode, string(buf))
	}

	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	var out ShowResponse
	if err := dec.Decode(&out); err != nil {
		return ShowResponse{}, err
	}
	return out, nil
}

// MaxContextLength returns the maximum context length reported by the model (if present).
//
// In /api/show, Ollama puts this in model_info as e.g. "qwen2.context_length".
func (s ShowResponse) MaxContextLength() (int, bool) {
	max := 0
	for k, v := range s.ModelInfo {
		if strings.HasSuffix(k, "context_length") {
			if n, ok := util.ToInt(v); ok {
				if n > max {
					max = n
				}
			}
		}
	}
	if max > 0 {
		return max, true
	}
	return 0, false
}

// TokensPerImage returns the model's tokens-per-image if exposed in model_info.
// If missing, returns (0,false).
func (s ShowResponse) TokensPerImage() (int, bool) {
	max := 0
	for k, v := range s.ModelInfo {
		// Common keys include: "llava.mm.tokens_per_image".
		if strings.Contains(k, "tokens_per_image") {
			if n, ok := util.ToInt(v); ok {
				if n > max {
					max = n
				}
			}
		}
	}
	if max > 0 {
		return max, true
	}
	return 0, false
}

func ioReadAllLimit(r io.Reader, max int64) ([]byte, error) {
	buf := &bytes.Buffer{}
	if max <= 0 {
		return io.ReadAll(r)
	}
	_, err := io.CopyN(buf, r, max+1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	b := buf.Bytes()
	if int64(len(b)) > max {
		return b[:max], nil
	}
	return b, nil
}
