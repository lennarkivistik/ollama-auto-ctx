# Architecture

This document explains **how the proxy works end-to-end** (request → decision → upstream → streaming response → calibration).

If you just want to run it, start with the README. If you want to change how `num_ctx` is computed, this is the doc.

---

## High-Level Flow

```
┌──────────┐     ┌─────────────────────────────────────────────────────┐     ┌────────┐
│  Client  │────▶│                   Proxy                              │────▶│ Ollama │
└──────────┘     │  ┌─────────────────────────────────────────────────┐ │     └────────┘
                 │  │ 1. Health check / CORS / Observability routing  │ │
                 │  │ 2. Start tracking (if enabled)                  │ │
                 │  │ 3. Parse request body                           │ │
                 │  │ 4. Extract features (text, messages, images)    │ │
                 │  │ 5. Fetch model limits (/api/show, cached)       │ │
                 │  │ 6. Estimate prompt tokens                       │ │
                 │  │ 7. Budget output tokens                         │ │
                 │  │ 8. Apply headroom + bucketize + clamp           │ │
                 │  │ 9. Inject/adjust options.num_ctx                │ │
                 │  │10. Forward via ReverseProxy                     │ │
                 │  └─────────────────────────────────────────────────┘ │
                 │                        │                             │
                 │  ┌─────────────────────▼─────────────────────────┐   │
                 │  │              Response Tap                      │   │
                 │  │ • Forward bytes immediately (streaming safe)   │   │
                 │  │ • Track first byte + progress                  │   │
                 │  │ • Feed loop detector (if enabled)              │   │
                 │  │ • Extract prompt_eval_count for calibration    │   │
                 │  └───────────────────────────────────────────────┘   │
                 │                        │                             │
                 │  ┌─────────────────────▼─────────────────────────┐   │
                 │  │             Supervisor (opt-in)                │   │
                 │  │ • Watchdog: timeout monitoring                 │   │
                 │  │ • Loop detection: n-gram analysis              │   │
                 │  │ • Restart hook: failure tracking               │   │
                 │  │ • Event bus: SSE publishing                    │   │
                 │  └───────────────────────────────────────────────┘   │
                 └──────────────────────────────────────────────────────┘
```

---

## Streaming Behavior (NDJSON)

Ollama streams partial results using **HTTP chunked transfer** with **newline-delimited JSON (NDJSON)**.

This proxy does **not** buffer streamed output. It forwards bytes immediately and parses only what it needs:
- If streaming: scans for `\n` boundaries and parses each JSON line
- Extracts `prompt_eval_count` from the final response for calibration
- Stops parsing after getting what it needs

**Result:** Latency is the same as upstream.

---

## Context Sizing Formula

At rewrite time, the proxy computes:

```
prompt_tokens_est = FixedOverhead
                  + PerMessageOverhead × messageCount
                  + TokensPerByte × textBytes
                  + imageTokens

output_budget = options.num_predict (if provided, clamped)
             OR dynamic_default (if DYNAMIC_DEFAULT_OUTPUT_BUDGET=true)
             OR DEFAULT_OUTPUT_BUDGET

needed          = prompt_tokens_est + output_budget
needed_headroom = ceil(needed × HEADROOM)

num_ctx = smallest bucket >= needed_headroom
num_ctx = clamp(num_ctx, MIN_CTX, min(MAX_CTX, model_max, safe_max))
```

### Why This Approach?

Exact tokenization is model- and template-dependent. Instead of shipping tokenizers, the proxy uses a cheap approximation that becomes accurate over time via calibration.

---

## Component Details

### Request Tracking (`internal/supervisor/tracker.go`)

Maintains lifecycle state for each request:

```go
type RequestInfo struct {
    ID, Endpoint, Model, Stream
    StartTime, FirstByteTime, LastActivityTime
    BytesForwarded, Status, Error
}
```

- **In-flight map:** Currently active requests
- **Recent buffer:** O(1) circular buffer of completed requests
- **Thread-safe:** Uses `sync.RWMutex`

### Watchdog (`internal/supervisor/watchdog.go`)

Monitors in-flight requests and cancels those exceeding timeouts:

| Timeout | Condition |
|---------|-----------|
| **TTFB** | No bytes received within timeout after start |
| **Stall** | No activity after first byte was received |
| **Hard** | Total duration exceeds limit |

- Runs in separate goroutine
- Checks every 1 second
- Uses context cancellation
- Fail-open: panics are recovered

### Loop Detection (`internal/supervisor/loopdetect.go`)

Detects repeating output patterns:

1. Maintains rolling window of recent output bytes
2. Tracks n-gram frequencies
3. Triggers when any n-gram exceeds threshold
4. Only activates after minimum output threshold

**Fail-open:** If parsing fails, detection doesn't trigger.

### Retry Logic (`internal/supervisor/retry.go`)

For non-streaming requests only:

1. Check eligibility (non-streaming, correct endpoint)
2. Execute request with timeout
3. On retriable error (5xx, connection error): wait backoff, retry
4. On success or max attempts: return result

**Response buffering:** Limited to configured max size.

### Restart Hook (`internal/supervisor/restart.go`)

Triggers Ollama restart on repeated failures:

1. Track consecutive timeout count
2. When threshold reached, execute restart command
3. Apply cooldown and rate limiting
4. Execute asynchronously (doesn't block requests)

**Safety guards:**
- Cooldown between restarts
- Max restarts per hour
- Only one restart at a time

### Metrics Collector (`internal/supervisor/metrics.go`)

Maintains Prometheus metrics for monitoring:

- Counters: requests, bytes, tokens, timeouts, loops, limit exceeded
- Histograms: request duration
- Gauges: in-flight requests, upstream health

Metrics are updated atomically and exposed via `/metrics` endpoint.

### Health Checker (`internal/supervisor/healthcheck.go`)

Periodically checks Ollama upstream health:

1. Background goroutine pings `/api/tags` every interval
2. Updates atomic health status
3. Updates Prometheus gauge
4. Exposes status via `/healthz` endpoints

**Fail-open:** Health check failures don't affect request forwarding.

### Event Bus (`internal/supervisor/events.go`)

Publishes lifecycle events for SSE consumers:

- **Bounded channel:** Prevents unbounded memory
- **Non-blocking publish:** Uses `select` with `default`
- **Fan-out:** Multiple subscribers receive same events
- **Fail-open:** Full buffer = dropped events

Event types: `request_start`, `first_byte`, `progress`, `done`, `canceled`, `timeout_*`, `upstream_error`, `loop_detected`

### Response Tap (`internal/proxy/tap.go`)

Wraps response body for streaming observation:

```go
func (t *TapReadCloser) Read(p []byte) (int, error) {
    n, err := t.rc.Read(p)           // Read from upstream
    if n > 0 {
        t.tracker.MarkFirstByte()     // First byte tracking
        t.tracker.MarkProgress(n)     // Progress tracking
        t.loopDetector.Feed(data)     // Loop detection
        t.checkOutputLimit()         // Output token limiting
        t.parseCalibration(data)      // Calibration extraction
    }
    return n, err                     // Forward immediately
}
```

**Output Token Limiting:**
- Estimates tokens from bytes using calibration
- Only checks after minimum output threshold
- Cancel mode: cancels request when limit exceeded
- Warn mode: logs warning but continues
- Fail-open: estimation errors don't trigger limit

---

## Observability Endpoints

### `GET /debug/requests`

Returns JSON snapshot:

```json
{
  "in_flight": {
    "123": {
      "id": "123",
      "endpoint": "/api/chat",
      "model": "llama2",
      "start_time": "...",
      "bytes_forwarded": 1024,
      "estimated_output_tokens": 256
    }
  },
  "recent": [...]
}
```

### `GET /events`

Server-Sent Events stream:

```
: connected

data: {"type":"request_start","request_id":"123",...}

data: {"type":"first_byte","request_id":"123","ttfb_ms":1500,...}

data: {"type":"progress","request_id":"123","bytes_out":2048,...}

data: {"type":"done","request_id":"123","status":"success",...}
```

### `GET /metrics`

Prometheus-format metrics:

```
# HELP ollama_proxy_requests_total Total number of requests processed
# TYPE ollama_proxy_requests_total counter
ollama_proxy_requests_total{endpoint="chat",model="llama2",status="success"} 42

# HELP ollama_proxy_request_duration_seconds Request duration in seconds
# TYPE ollama_proxy_request_duration_seconds histogram
ollama_proxy_request_duration_seconds_bucket{endpoint="chat",model="llama2",le="0.005"} 10
...

# HELP ollama_proxy_upstream_healthy Upstream Ollama health status
# TYPE ollama_proxy_upstream_healthy gauge
ollama_proxy_upstream_healthy 1
```

### `GET /healthz`

Combined proxy + upstream health:

- Returns `200 OK` with "ok" if proxy and upstream are healthy
- Returns `503 Service Unavailable` with "upstream unhealthy" if upstream is down (when health check enabled)

### `GET /healthz/upstream`

Upstream-only health with details:

```json
{
  "healthy": true,
  "last_check": "2024-01-11T22:30:00Z",
  "last_error": ""
}
```

---

## Package Responsibilities

| Package | Responsibility |
|---------|----------------|
| `cmd/ollama-auto-ctx` | Entry point, wiring, graceful shutdown |
| `internal/config` | Env/flag parsing, validation |
| `internal/proxy` | HTTP handler, reverse proxy, tap, endpoints |
| `internal/estimate` | Feature extraction, token estimation, bucket logic |
| `internal/calibration` | Per-model EMA learning, persistence |
| `internal/ollama` | `/api/show` client, caching |
| `internal/supervisor` | Tracking, watchdog, loop detection, retry, restart, events, metrics, health check |

---

## Data Flow Examples

### Small Chat Request

1. Extract: model, 1 message, 50 bytes text, no images
2. Lookup model max context via cached `/api/show`
3. Use calibration params for model (or defaults)
4. Compute: 50 × 0.25 + 8 + 32 = ~52 prompt tokens
5. Add output budget (1024), apply headroom (1.25)
6. Bucketize → 2048
7. Inject `options.num_ctx`
8. Stream response, extract `prompt_eval_count`
9. Update calibration EMA

### Large Multimodal Request

Same flow, but:
- `imageTokens` from model metadata (`tokens_per_image`)
- Final `num_ctx` clamped to model's max context
- Larger bucket selected

### Timeout Scenario

1. Request starts, tracked
2. No response bytes within TTFB timeout
3. Watchdog cancels context
4. Request marked as `timeout_ttfb`
5. Event published
6. Restart hook notified (if enabled)

---

## Where to Change Things

| Change | Location |
|--------|----------|
| Bucket selection | `internal/estimate/estimate.go` → `Bucketize()` |
| Estimation formula | `internal/estimate/estimate.go`, `internal/calibration/store.go` |
| Rewritten endpoints | `internal/proxy/handler.go` → `ServeHTTP()` |
| Model metadata extraction | `internal/ollama/client.go` |
| Timeout behavior | `internal/supervisor/watchdog.go` |
| Loop detection tuning | `internal/supervisor/loopdetect.go` |
| Retry policy | `internal/supervisor/retry.go` |
| Restart conditions | `internal/supervisor/restart.go` |
| Metrics exposed | `internal/supervisor/metrics.go` |
| Health check behavior | `internal/supervisor/healthcheck.go` |
| Output limit behavior | `internal/proxy/tap.go` → `Read()` |
