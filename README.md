**ollama-auto-ctx** is a small Go reverse proxy that sits in front of Ollama and automatically injects an appropriate `options.num_ctx` for each request.

It targets a very specific pain: **silent prompt truncation** caused by low default context windows, especially once you add a system prompt, tools, and chat history.

---

## What it does

- **Acts as a 1:1 reverse proxy** for the Ollama HTTP API (future‑proof pass‑through).
- **Intercepts only**:
  - `POST /api/chat`
  - `POST /api/generate`
- **Estimates required context** and injects/adjusts `options.num_ctx` based on:
  - prompt size (system + messages + tools)
  - expected output budget (`options.num_predict`, if present)
  - a configurable headroom multiplier
  - **bucket selection** (small prompts stay fast)
  - the model’s max context from `POST /api/show`
- **Preserves streaming**: Ollama streams newline‑delimited JSON (NDJSON); the proxy forwards bytes immediately.
- **Auto‑calibrates over time** by reading Ollama’s `prompt_eval_count` and improving its estimation parameters per model.

---

## Quick start

### Option 1: Manual build & run

```bash
go build -o ollama-auto-ctx ./cmd/ollama-auto-ctx

# upstream ollama: http://localhost:11434
# proxy:          http://localhost:11435
./ollama-auto-ctx
```

### Option 2: Systemd service (recommended for production)

```bash
# Install as systemd service (requires root)
sudo ./install.sh install

# Or install with custom binary
sudo ./install.sh install --binary-path ./ollama-auto-ctx

# Or use existing binary without building
sudo ./install.sh install --no-build

# Start the service
sudo systemctl start ollama-auto-ctx

# Check status
sudo systemctl status ollama-auto-ctx

# Uninstall (includes cleanup prompts)
sudo ./install.sh uninstall
```

The installer creates:
- Service user and directories
- Systemd service file
- Configuration file at `/etc/ollama-auto-ctx/environment`
- Calibration persistence (survives restarts)

**Installation Options:**
- `--binary-path PATH`: Use existing binary instead of building
- `--no-build`: Use existing binary in install directory
- `--service-name NAME`: Custom service name
- `--listen-addr ADDR`: Custom listen address
- `--upstream-url URL`: Custom Ollama URL

Point clients (n8n, OpenWebUI, custom code) at:

- `http://localhost:11435` *(proxy)*

instead of:

- `http://localhost:11434` *(ollama)*

### Option 3: Docker

```bash
docker build -t ollama-auto-ctx .
docker run --rm -p 11435:11435 \
  -e UPSTREAM_URL=http://host.docker.internal:11434 \
  ollama-auto-ctx
```

---

## How `num_ctx` is chosen

For `/api/chat` and `/api/generate`, the proxy uses a simple, fast model:

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

Then it applies an override policy (see `OVERRIDE_NUM_CTX`): by default it **only increases** an existing `options.num_ctx` if it looks too small.

> **Important:** The proxy uses `options.num_predict` from your request (if provided) to estimate the output budget. If you want a **large output response**, you have two options:
> 1. **Explicitly set `options.num_predict`** in your request (always respected)
> 2. **Enable `DYNAMIC_DEFAULT_OUTPUT_BUDGET=true`** to automatically adjust the default budget based on prompt size (useful for workflows like n8n that generate large JSON outputs)
> 
> Without either, the proxy defaults to `DEFAULT_OUTPUT_BUDGET` (1024 tokens), which may truncate very long responses.

> Want the deep dive? See **docs/ARCHITECTURE.md** and **docs/DESIGN_DECISIONS.md**.

---

## Buckets (performance vs. precision)

Buckets are the key to keeping “hello world” fast while still handling large prompts safely.

- If you use **huge** `num_ctx` for everything, you may pay unnecessary KV‑cache cost for tiny requests.
- If you use **tiny** `num_ctx`, you’ll truncate system prompt/history and get erratic behavior.

The proxy solves this by rounding up to the **smallest** bucket that fits.

**Default buckets (fine-grained for precise allocation):**

```
1024,2048,4096,8192,9216,10240,11264,12288,13312,14336,15360,16384,20480,24576,28672,32768,36864,40960,45056,49152,53248,57344,61440,65536,69632,73728,77824,81920,86016,90112,94208,98304,102400
```

These buckets provide fine-grained control to minimize over-allocation while still keeping small prompts fast. The default range goes from 1K to 100K tokens.

**Example: simpler buckets (fewer options, easier to manage):**

```bash
BUCKETS=2048,4096,8192,16384,32768,65536
```

Tip: finer buckets reduce "jump" overhead (e.g. 9000 → 10240 instead of 16384). The trade-off is maintaining a longer bucket list.

---

## Configuration

Configuration works via **environment variables** or **flags** (flags override env).

Common env vars:

- `LISTEN_ADDR` (default `:11435`)
- `UPSTREAM_URL` (default `http://localhost:11434`)

Context sizing:

- `MIN_CTX` (default `1024`)
- `MAX_CTX` (default `81920`)
- `BUCKETS` (default `1024,2048,4096,8192,9216,10240,11264,12288,13312,14336,15360,16384,20480,24576,28672,32768,36864,40960,45056,49152,53248,57344,61440,65536,69632,73728,77824,81920,86016,90112,94208,98304,102400`)
- `HEADROOM` (default `1.25`)

Output budgeting:

- `DEFAULT_OUTPUT_BUDGET` (default `1024`) — used when `options.num_predict` is not provided in the request
- `MAX_OUTPUT_BUDGET` (default `10240`) — maximum output budget (clamps `options.num_predict` if provided)
- `DYNAMIC_DEFAULT_OUTPUT_BUDGET` (default `false`) — if `true`, automatically adjusts default output budget based on prompt size

**Dynamic Default Output Budget:**

When `DYNAMIC_DEFAULT_OUTPUT_BUDGET=true` and `options.num_predict` is **not** set in the request, the proxy computes:
```
dynamic_default = max(DEFAULT_OUTPUT_BUDGET, 256 + prompt_tokens_est/2)
output_budget = min(dynamic_default, MAX_OUTPUT_BUDGET)
```

This helps with workflows (like n8n) that generate large JSON outputs without explicitly setting `num_predict`. Small prompts stay near `DEFAULT_OUTPUT_BUDGET` (fast), while larger prompts automatically reserve more room for output.

**Examples:**
- Prompt ≈ 600 tokens: dynamic default ≈ 556 (minimal change from 1024)
- Prompt ≈ 10,000 tokens: dynamic default ≈ 5,256 (much safer than fixed 1024)

The proxy logs the output budget source in each `ctx decision` log entry as `output_budget_source`, showing whether it came from:
- `"explicit_num_predict"` — user provided `options.num_predict`
- `"dynamic_default"` — computed using dynamic algorithm
- `"fixed_default"` — used fixed `DEFAULT_OUTPUT_BUDGET`

> **Note:** If you explicitly set `options.num_predict` in your request, it always takes precedence (dynamic logic is bypassed). For example:
> ```json
> {
>   "model": "llama3",
>   "messages": [...],
>   "options": {
>     "num_predict": 8192
>   }
> }
> ```
> The proxy will include this in its context size calculation, ensuring `num_ctx` is large enough for both the prompt and the expected output.

Override behavior:

- `OVERRIDE_NUM_CTX` = `always|if_missing|if_too_small` (default `if_too_small`)

Calibration:

- `CALIBRATION_ENABLED` (default `true`)
- `CALIBRATION_FILE` (optional; if set, calibration survives restarts)

Example:

```bash
LISTEN_ADDR=:11435 \
UPSTREAM_URL=http://localhost:11434 \
MIN_CTX=1024 \
MAX_CTX=65536 \
BUCKETS=2048,4096,8192,16384,32768,65536 \
DYNAMIC_DEFAULT_OUTPUT_BUDGET=true \
CALIBRATION_FILE=./data/calibration.json \
./ollama-auto-ctx
```

---

## Health

- `GET /healthz` → `200 ok`

---

## Notes / limitations

- Estimation starts as a **best-effort heuristic**, then improves via calibration.
- This proxy does **not** translate OpenAI `/v1/*` endpoints into Ollama endpoints — it forwards them unchanged.
- No auth by default; assume a trusted network or put it behind a gateway.
- **Output size:** The proxy cannot know your desired output length. If you need large responses:
  - Explicitly set `options.num_predict` in your request (always respected), or
  - Enable `DYNAMIC_DEFAULT_OUTPUT_BUDGET=true` to automatically adjust based on prompt size
  - Without either, the default output budget (1024 tokens) is used, which may cause truncation for very long outputs

---

## Docs

- **docs/ARCHITECTURE.md** — end-to-end request flow + components
- **docs/DESIGN_DECISIONS.md** — “why this design” and trade-offs
- **docs/OPERATIONS.md** — perf, deployment, security, and ops notes
- **docs/AGENTS.md** — a “map of the repo” for AI agents and contributors


---

## License

Apache License, Version 2.0 (http://www.apache.org/licenses/LICENSE-2.0)
