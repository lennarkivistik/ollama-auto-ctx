# Design Decisions & Trade-offs

This is the "why" behind the implementation choices.

---

## 1) Preserve Ollama behavior unless we *must* intervene

The proxy forwards all endpoints 1:1 and only rewrites `POST /api/chat` and `POST /api/generate`.

**Why:** Minimal blast radius + future-proofing. Unknown endpoints and fields pass through unchanged.

---

## 2) Don't break streaming

Ollama uses HTTP chunked streaming with newline-delimited JSON (NDJSON). A proxy can easily break this if it buffers or re-encodes.

**Decision:** Use Go's `httputil.ReverseProxy` and a response-body wrapper that parses NDJSON incrementally while forwarding bytes immediately.

---

## 3) Token estimation must be fast

Exact tokenization is model-specific and often depends on hidden prompt templates.

**Decision:** Start with a cheap estimation model based on text bytes + message counts + image counts:

```
tokens ≈ FixedOverhead + PerMessageOverhead×messages + TokensPerByte×bytes + imageTokens
```

---

## 4) Calibration beats hard-coded constants

Ollama responses provide `prompt_eval_count` (actual prompt tokens). That's "ground truth".

**Decision:** After each request, update per-model estimation parameters using an exponential moving average (EMA). This gradually replaces user-tuned constants with model-specific learned values.

---

## 5) Buckets keep small prompts fast

Your original requirement: *don't force everything to 16k* just because some requests are large.

**Decision:** Round up to a configured bucket list. Smallest bucket ≥ required tokens.

Powers-of-two are common, but finer buckets (e.g., +1024 steps) can reduce over-allocation.

---

## 6) Always respect model limits from `/api/show`

A computed context larger than the model supports is pointless (and can fail).

**Decision:** Fetch `/api/show` for the request model (cached) and clamp to its max context length.

---

## 7) Override policy: don't stomp explicit user intent

Some clients set `options.num_ctx` intentionally.

**Decision:** Configurable override policy:
- `always` — proxy always sets it
- `if_missing` — proxy sets only if missing
- `if_too_small` *(default)* — proxy increases only if below estimated need

---

## 8) Robustness edge cases

Design favors "don't become a strict gateway":
- If body is too large → pass through
- If JSON parse fails → pass through
- If content-type is non-JSON → pass through
- For multimodal: never count base64 bytes as text tokens

---

## 9) Supervisor features: monitoring auto-enabled, protection opt-in, all fail-open

**Decision:**
- **Monitoring features** (tracking, observability, metrics, health checks) are auto-enabled when `SUPERVISOR_ENABLED=true`
- **Protection features** (watchdog, loop detection, retry, restart, output limits) remain explicitly opt-in
- All supervisor features fail-open: internal errors never block or break the request path

**Why monitoring auto-enabled:** Basic monitoring is essential for understanding proxy behavior and doesn't change request handling. Low overhead, high value.

**Why protection opt-in:** Safety timeouts, loop detection, and restarts are significant behavioral changes. Users must explicitly enable them after testing.

**Why fail-open:** The proxy's primary job is forwarding requests. Supervisor failures should never become a new source of request failures.

---

## 10) Watchdog monitors, doesn't modify

The watchdog observes request lifecycle via tracker state and only acts when timeouts are exceeded.

**Implementation notes:**
- Runs in separate goroutine with panic recovery
- Uses context cancellation (Go's standard pattern)
- Checks every 1 second (balances responsiveness vs CPU)
- Only intervenes when requests would otherwise run forever

---

## 11) Loop detection is conservative

Models can legitimately repeat themselves (lists, code, etc.). False positives are worse than missed loops.

**Decision:** Conservative n-gram detection with:
- Minimum output threshold before activation
- Tunable n-gram size and repeat count
- Only triggers on clear repetition patterns

**Why opt-in:** This is a heuristic that may need tuning per model/use case.

---

## 12) Retry only for non-streaming requests

Retrying a streaming response after partial bytes were sent would break the 1:1 API semantics — the client would see duplicate partial content.

**Decision:** By default, only retry non-streaming requests. Streaming requests get a single attempt.

**Why this is safe:** Non-streaming responses are fully buffered before returning, so we can safely retry without client seeing partial results.

---

## 13) Restart hook has multiple safety guards

Restarting Ollama is disruptive (kills in-flight requests). Uncontrolled restarts could cause restart loops.

**Decision:** Multiple guards:
- Cooldown between restarts (default 120s)
- Rate limit per hour (default 3)
- Serialized execution (only one restart at a time)
- Require explicit command configuration
- Async execution (doesn't block request handling)

**Failure mode:** If restart command fails, it's logged but doesn't affect proxy operation.

---

## 14) Observability is non-intrusive

GUI monitoring should never affect request processing.

**Decision:**
- Event bus uses bounded, non-blocking publish
- Slow consumers cause dropped events (not backpressure)
- Progress events are throttled to reduce overhead
- Endpoints only read tracker state

---

## 15) Circular buffer for O(1) request history

The recent request buffer was initially O(n) for insertions due to array shifting.

**Decision:** Use a proper circular buffer with head/tail indices for O(1) insertions. With default buffer size of 200, this eliminates 199 element copies per finished request.

---

## 16) Deliberate non-goals

Kept out on purpose:
- Hardware probing for safe maximum context (cross-platform complexity)
- Translating OpenAI `/v1/*` endpoints (different project scope)
- Semantic loop detection (too expensive, keep it cheap)
- Retry for streaming requests (breaks 1:1 semantics)
- Process management beyond command execution (not a process supervisor)

---

## 17) Ops-friendly configuration

Env vars + flags (flags override env) to support:
- Local development
- Container deployment
- Simple automation

All config is validated at startup with clear error messages.
