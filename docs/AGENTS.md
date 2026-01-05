# Agent / contributor quick map

This file is written for:
- future contributors skimming the repo
- AI coding agents (Cursor, Copilot, etc.) that need fast orientation

---

## One-sentence goal

Keep the Ollama API 1:1, but automatically choose a safe + fast `options.num_ctx` for `/api/chat` and `/api/generate`.

---

## “Golden rules” of this codebase

1. **Do not break streaming** (NDJSON must remain byte-for-byte)
2. **Keep the interception surface small**
3. **Clamp to model limits from `/api/show`**
4. **Prefer calibration over hard-coded constants**
5. **Fail open** (if parsing fails, pass through)

---

## Where the important logic lives

- **Request rewriting + reverse proxy plumbing**
  - `internal/proxy/handler.go`

- **Streaming NDJSON parsing (calibration tap)**
  - `internal/proxy/tap.go`

- **Token estimation + bucket selection + clamping**
  - `internal/estimate/estimate.go`

- **Per-model calibration (EMA learning)**
  - `internal/calibration/store.go`

- **Model metadata (`/api/show`) and caching**
  - `internal/ollama/client.go`
  - `internal/ollama/showcache.go`

- **Config defaults + env/flag parsing**
  - `internal/config/config.go`

---

## Common edits

### Change bucket strategy
- Update env var `BUCKETS` in your deployment
- Or adjust the default in `internal/config/config.go`
- Bucket selection code lives in `internal/estimate/estimate.go` (`Bucketize`)

### Add support for a new endpoint that uses `num_ctx`
- Add it to the rewrite routing in `internal/proxy/handler.go`
- Implement a small feature extractor in `internal/estimate/estimate.go`
- Keep it tolerant to unknown fields (use map-based JSON where needed)

### Improve estimation
- Improve feature extraction (more accurate “text bytes” counting)
- Improve output budget logic
- Keep it cheap; calibration should do the heavy lifting over time

---

## Testing pointers

- Estimation tests: `internal/estimate/*_test.go`
- Proxy handler tests: `internal/proxy/*_test.go`

If you add streaming behavior, add tests to ensure NDJSON is not buffered or re-chunked.
