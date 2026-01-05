# Design decisions & trade-offs

This is the “why” behind the implementation choices.

---

## 1) Preserve Ollama behavior unless we *must* intervene

The proxy forwards all endpoints 1:1 and only rewrites `POST /api/chat` and `POST /api/generate`.

**Why:** minimal blast radius + future-proofing. Unknown endpoints and fields are passed through unchanged.

---

## 2) Don’t break streaming

Ollama uses HTTP chunked streaming with newline-delimited JSON (NDJSON). A proxy can easily break this if it buffers or re-encodes.

**Decision:** use Go’s `httputil.ReverseProxy` and a response-body wrapper that parses NDJSON incrementally while forwarding bytes immediately.

---

## 3) Token estimation must be fast

Exact tokenization is model-specific and often depends on hidden prompt templates.

**Decision:** start with a cheap estimation model based on text bytes + message counts + image counts:

```
FixedOverhead + PerMessageOverhead*messageCount + TokensPerByte*textBytes + imageTokens
```

---

## 4) Calibration beats hard-coded constants

Ollama responses provide `prompt_eval_count` (actual prompt tokens evaluated). That’s “ground truth”.

**Decision:** after each request, update per-model estimation parameters using an exponential moving average (EMA).

This gradually replaces user-tuned constants with model-specific learned values.

---

## 5) Buckets keep small prompts fast

Your original requirement: *don’t force everything to 16k* just because some requests are large.

**Decision:** round up to a configured bucket list: smallest bucket ≥ required tokens.

### Should buckets be powers-of-two?

Not necessarily.

- **Powers-of-two** are easy to reason about and match common memory allocation patterns.
- **Finer buckets** (e.g. +1024 steps) can reduce over-allocation around the 8–16k region (e.g. 9000 → 10240 instead of 16384).

In practice: for a homelab, finer buckets can be a nice win. The proxy treats buckets as an ordered list; you can provide any list that makes sense for your hardware.

---

## 6) Always respect model limits from `/api/show`

A computed context larger than the model supports is pointless (and can fail).

**Decision:** fetch `/api/show` for the request model (cached) and clamp to its max context length.

---

## 7) Override policy: don’t stomp explicit user intent

Some clients set `options.num_ctx` intentionally.

**Decision:** configurable override policy:
- `always` — proxy always sets it
- `if_missing` — proxy sets only if missing
- `if_too_small` *(default)* — proxy increases only if it’s below estimated need

---

## 8) Robustness edge cases

Design favors “don’t become a strict gateway”:

- if body is too large → pass through
- if JSON parse fails → pass through
- if content-type is non-JSON → pass through
- for multimodal payloads: never count base64 bytes as text tokens; count images using metadata where possible

---

## 9) Deliberate non-goals

Kept out on purpose:
- Hardware probing for a safe maximum context (cross-platform complexity)
- Translating OpenAI `/v1/*` endpoints into Ollama endpoints (different project scope)

---

## 10) Ops-friendly configuration

Env vars + flags (flags override env) to support:
- local development
- container deployment
- simple automation
