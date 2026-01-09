# Architecture

This document explains **how the proxy works end-to-end** (request → decision → upstream → streaming response → calibration).

If you just want to run it, start with the README. If you want to change how `num_ctx` is computed, this is the doc.

---

## High-level flow

1. Client sends a request to the proxy.
2. Proxy checks for:
   - `/healthz`
   - CORS preflight
3. For normal traffic, the proxy behaves like a standard reverse proxy **except** for:
   - `POST /api/chat`
   - `POST /api/generate`
4. For those two endpoints, the proxy:
   - reads and parses the JSON body (bounded by a max size)
   - extracts features (text bytes, message count, image count, requested output tokens, existing `num_ctx`, etc.)
   - fetches model metadata via `/api/show` (cached) to learn max context and image token cost
   - estimates needed prompt tokens
   - budgets expected output tokens
   - applies headroom + bucketization + clamping
   - decides whether to override user-provided `num_ctx` based on policy
   - forwards the modified request to Ollama
5. The response is forwarded **byte-for-byte**, with a “tap” that observes `prompt_eval_count` to auto-calibrate.

---

## Streaming behavior (NDJSON)

Ollama streams partial results using **HTTP chunked transfer** with **newline-delimited JSON (NDJSON)**.

This proxy does **not** buffer streamed output. It forwards bytes immediately and parses only what it needs:
- if streaming: it scans for `\n` boundaries and attempts to parse each JSON line
- when it sees the final `done=true` object (or a final JSON object), it extracts `prompt_eval_count`
- it stops parsing after it has what it needs

This keeps latency the same as upstream.

---

## Core context sizing formula

At rewrite time, the proxy computes:

```
prompt_tokens_est = FixedOverhead
                 + PerMessageOverhead * messageCount
                 + TokensPerByte      * textBytes
                 + imageTokens

output_budget   = options.num_predict (clamped) 
                 OR (if DYNAMIC_DEFAULT_OUTPUT_BUDGET=true: max(DEFAULT_OUTPUT_BUDGET, 256 + prompt_tokens_est/2))
                 OR DEFAULT_OUTPUT_BUDGET
needed          = prompt_tokens_est + output_budget
needed_headroom = ceil(needed * HEADROOM)

num_ctx  = smallest bucket >= needed_headroom
num_ctx  = clamp(num_ctx, MIN_CTX, min(MAX_CTX, model_max_ctx, safe_max_ctx))
```

### Why this "heuristic + calibration" approach?

Exact tokenization is model- and template-dependent. Instead of shipping tokenizers and trying to replicate Ollama's internal prompt rendering, the proxy uses a cheap approximation that becomes accurate over time by learning from `prompt_eval_count`.

---

## Thinking Mode Support

The proxy supports model-specific thinking modes via `__think=<verdict>` directives in system prompts.

### Detection Method

**System Prompt Directive**
- Detects `__think=<verdict>` anywhere in system prompts
- Automatically strips it from the prompt before sending to Ollama
- Works for both `/api/generate` (system field) and `/api/chat` (system role messages)

### Supported Model Families

- qwen3, deepseek: boolean (true/false) → `think` = bool
- gpt-oss: level (low/medium/high) → `think` = string

### Flow Example

1. Client sends request:
```json
{
  "model": "gpt-oss:20b",
  "messages": [
    {"role": "system", "content": "You are helpful. __think=high"}
  ]
}
```

2. Proxy extracts `__think=high` from system prompt
3. Proxy validates `high` is valid for gpt-oss family
4. Proxy removes `__think=high` from the system message content
5. Proxy injects `"think": "high"` at top level of request
6. Proxy forwards cleaned request to Ollama

Result:
```json
{
  "model": "gpt-oss:20b",
  "messages": [{"role": "system", "content": "You are helpful."}],
  "think": "high",
  "options": {"num_ctx": 8192}
}
```

Invalid verdicts or unsupported model families are silently ignored (fail-open).

---

## Repo map (important packages)

### `cmd/ollama-auto-ctx/main.go`
Entry point:
- loads config
- constructs dependencies (Ollama client, show cache, calibration store, handler)
- starts the HTTP server + graceful shutdown

### `internal/proxy/handler.go`
Main HTTP handler:
- routes health/CORS
- detects whether a request is eligible for rewrite
- rewrites request body when possible
- forwards via `httputil.ReverseProxy`
- attaches the streaming “tap” in the reverse-proxy response modifier

### `internal/proxy/tap.go`
Streaming-safe response wrapper:
- implements `io.ReadCloser`
- forwards bytes to the client
- parses NDJSON/JSON in a bounded way
- extracts `prompt_eval_count` for calibration

### `internal/estimate/estimate.go`
All estimation math:
- feature extraction for `/api/chat` and `/api/generate`
- prompt token estimation
- output token budgeting
- headroom, bucketization, and clamping helpers

### `internal/calibration/store.go`
Per-model learning:
- stores per-model parameters (tokens/byte, fixed overhead, per-message overhead)
- updates them using an EMA from observed `prompt_eval_count`
- optionally persists to disk

### `internal/ollama/*`
Model metadata:
- `/api/show` client + parsing helpers
- caching layer (TTL)

### `internal/config/config.go`
Configuration:
- env + flags
- validation (buckets ascending, limits sane)

---

## Data flow examples

### Small chat request

1. Extract: model, 1 message, 5 bytes of text, no images
2. Lookup model max context via cached `/api/show`
3. Use current calibration params for the model
4. Compute needed tokens, add output budget, apply headroom
5. Bucketize → 2048
6. Inject/adjust `options.num_ctx`
7. Stream response back
8. Observe `prompt_eval_count` and refine calibration slightly

### Large multimodal request

Same flow, but `imageTokens` comes from model metadata (`tokens_per_image`), and the final `num_ctx` is clamped to the model’s actual max context.

---

## Where to change things

- **Bucket selection / increments**: `internal/estimate/estimate.go` (`Bucketize`)
- **Estimation formula knobs**: `internal/calibration/store.go` (defaults + EMA update) and `internal/estimate/estimate.go` (how features are translated into token counts)
- **Which endpoints are rewritten**: `internal/proxy/handler.go`
- **Model max context / tokens-per-image extraction**: `internal/ollama/client.go`
