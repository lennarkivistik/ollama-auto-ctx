# Agent / Contributor Quick Map

This file is written for:
- Future contributors skimming the repo
- AI coding agents (Cursor, Copilot, etc.) that need fast orientation

---

## One-Sentence Goal

Keep the Ollama API 1:1, but automatically choose a safe + fast `options.num_ctx` for `/api/chat` and `/api/generate`, with optional protection against runaway requests.

---

## Golden Rules

1. **Do not break streaming** — NDJSON must remain byte-for-byte
2. **Keep the interception surface small** — Only rewrite `/api/chat` and `/api/generate`
3. **Clamp to model limits** — Use `/api/show` to get max context
4. **Prefer calibration over hard-coded constants** — Learn from actual token counts
5. **Fail open** — If parsing fails, pass through unchanged
6. **Supervisor features are opt-in** — All protection features are disabled by default

---

## Package Map

```
cmd/ollama-auto-ctx/     # Entry point, wiring
internal/
├── calibration/         # Per-model learning from prompt_eval_count
│   └── store.go
├── config/              # Env vars + flags + validation
│   └── config.go
├── estimate/            # Token estimation + bucket selection
│   └── estimate.go
├── ollama/              # /api/show client + caching
│   ├── client.go
│   └── showcache.go
├── proxy/               # HTTP handler, reverse proxy, response tap
│   ├── handler.go       # Main ServeHTTP, rewriting, observability endpoints
│   └── tap.go           # Streaming NDJSON parser, calibration, loop detection
└── supervisor/          # Request tracking + protection features
    ├── tracker.go       # Request lifecycle tracking
    ├── events.go        # Event bus for SSE
    ├── watchdog.go      # Timeout monitoring
    ├── loopdetect.go    # Repeating output detection
    ├── retry.go         # Retry logic for non-streaming
    └── restart.go       # Ollama restart hook
```

---

## Where Things Live

### Core Context Sizing
- **Request rewriting**: `internal/proxy/handler.go` → `rewriteRequestIfPossible()`
- **Token estimation**: `internal/estimate/estimate.go` → `EstimatePromptTokens()`
- **Bucket selection**: `internal/estimate/estimate.go` → `Bucketize()`
- **Model limits**: `internal/ollama/client.go` → `ShowResponse.MaxContextLength()`

### Calibration
- **EMA learning**: `internal/calibration/store.go` → `Update()`
- **Streaming tap**: `internal/proxy/tap.go` → `tryParseJSON()`

### Protection Features
- **Request tracking**: `internal/supervisor/tracker.go`
- **Timeout detection**: `internal/supervisor/watchdog.go` → `checkTimeouts()`
- **Loop detection**: `internal/supervisor/loopdetect.go` → `Feed()`
- **Retry logic**: `internal/supervisor/retry.go` → `DoWithRetry()`
- **Restart hook**: `internal/supervisor/restart.go` → `RecordTimeout()`

### Observability
- **JSON endpoint**: `internal/proxy/handler.go` → `handleDebugRequests()`
- **SSE endpoint**: `internal/proxy/handler.go` → `handleSSEEvents()`
- **Event bus**: `internal/supervisor/events.go`

---

## Common Changes

### Change bucket strategy
1. Update `BUCKETS` env var, or
2. Change defaults in `internal/config/config.go`
3. Selection logic in `internal/estimate/estimate.go` → `Bucketize()`

### Add a new rewritten endpoint
1. Add routing in `internal/proxy/handler.go` → `ServeHTTP()`
2. Add feature extractor in `internal/estimate/estimate.go`
3. Keep tolerant to unknown fields

### Improve token estimation
1. Improve feature extraction in `internal/estimate/estimate.go`
2. Adjust defaults or calibration in `internal/calibration/store.go`

### Add a new supervisor feature
1. Create implementation in `internal/supervisor/`
2. Add config in `internal/config/config.go`
3. Wire in `cmd/ollama-auto-ctx/main.go`
4. Integrate with handler/tap as needed

---

## Testing Pointers

```
internal/estimate/estimate_test.go      # Token estimation
internal/proxy/handler_test.go          # Handler logic
internal/proxy/watchdog_test.go         # Watchdog integration
internal/proxy/observability_test.go    # SSE/JSON endpoints
internal/supervisor/*_test.go           # Supervisor components
```

**Critical:** Any streaming behavior change must verify NDJSON is not buffered or re-chunked.

---

## Configuration Summary

Enable the supervisor layer:
```bash
SUPERVISOR_ENABLED=true
```

When `SUPERVISOR_ENABLED=true`, these monitoring features are **automatically enabled**:
- `SUPERVISOR_TRACK_REQUESTS=true` - Request lifecycle tracking
- `SUPERVISOR_OBS_ENABLED=true` - Observability endpoints
- `SUPERVISOR_METRICS_ENABLED=true` - Prometheus metrics
- `SUPERVISOR_HEALTH_CHECK_ENABLED=true` - Health monitoring

Then enable protection features as needed:
- Watchdog: `SUPERVISOR_WATCHDOG_ENABLED=true`
- Loop detection: `SUPERVISOR_LOOP_DETECT_ENABLED=true`
- Retry: `SUPERVISOR_RETRY_ENABLED=true`
- Restart: `SUPERVISOR_RESTART_ENABLED=true` + `SUPERVISOR_RESTART_CMD=...`
- Output safety limiting: `SUPERVISOR_OUTPUT_SAFETY_LIMIT_ENABLED=true`

See README.md for complete configuration reference.
