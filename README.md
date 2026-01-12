# Ollama AutoCTX

A smart reverse proxy for Ollama that automatically manages context windows, prevents runaway requests, and provides real-time observability.

---

## Why This Exists

**Problem:** Ollama's default context window is often too small, causing silent prompt truncation. But setting it too large wastes memory and slows down small requests.

**Solution:** This proxy automatically chooses the right `options.num_ctx` based on your actual prompt size, with bucket-based allocation that keeps small prompts fast.

---

## Features

| Feature | Description | Default |
|---------|-------------|---------|
| **Auto Context Sizing** | Estimates needed context and injects optimal `num_ctx` | Always on |
| **Bucket Allocation** | Rounds up to predefined sizes for efficiency | Always on |
| **Auto-Calibration** | Learns from Ollama's actual token counts per model | Enabled |
| **Watchdog Timeouts** | Cancels stuck requests (TTFB/stall/hard timeouts) | Disabled |
| **Loop Detection** | Detects and cancels degenerate repeating output | Disabled |
| **Retry Logic** | Retries failed non-streaming requests | Disabled |
| **Restart Hook** | Restarts Ollama on repeated failures | Disabled |
| **Observability** | Real-time request monitoring via SSE/JSON endpoints | Disabled |
| **Prometheus Metrics** | Exposes Prometheus-format metrics for monitoring | Disabled |
| **Health Check** | Monitors upstream Ollama health status | Disabled |
| **Output Token Limiting** | Prevents runaway generations with configurable limits | Disabled |

---

## Quick Start

### Option 1: Build and Run

```bash
go build -o ollama-auto-ctx ./cmd/ollama-auto-ctx
./ollama-auto-ctx
```

### Option 2: Systemd Service

```bash
sudo ./install.sh install
sudo systemctl start ollama-auto-ctx
```

### Option 3: Docker

```bash
docker build -t ollama-auto-ctx .
docker run --rm -p 11435:11435 \
  -e UPSTREAM_URL=http://host.docker.internal:11434 \
  ollama-auto-ctx
```

**Point your clients at:** `http://localhost:11435` (instead of `:11434`)

---

## Configuration Reference

Configuration via environment variables or command-line flags (flags override env).

### Core Settings

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `LISTEN_ADDR` | `--listen` | `:11435` | Proxy listen address |
| `UPSTREAM_URL` | `--upstream` | `http://localhost:11434` | Ollama upstream URL |
| `LOG_LEVEL` | `--log-level` | `info` | Log level (debug/info/warn/error) |

### Context Window Sizing

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `MIN_CTX` | `--min-ctx` | `1024` | Minimum context size |
| `MAX_CTX` | `--max-ctx` | `81920` | Maximum context size |
| `BUCKETS` | `--buckets` | `1024,2048,...102400` | Comma-separated context buckets |
| `HEADROOM` | `--headroom` | `1.25` | Safety multiplier for token estimates |
| `OVERRIDE_NUM_CTX` | `--override-policy` | `if_too_small` | Override policy: `always`, `if_missing`, `if_too_small` |

### Output Budgeting

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `DEFAULT_OUTPUT_BUDGET` | `--default-output` | `1024` | Default output tokens when `num_predict` not set |
| `MAX_OUTPUT_BUDGET` | `--max-output` | `10240` | Maximum output token budget |
| `STRUCTURED_OVERHEAD` | `--structured-overhead` | `128` | Extra tokens for JSON/structured output |
| `DYNAMIC_DEFAULT_OUTPUT_BUDGET` | `--dynamic-default-output` | `false` | Adjust default budget based on prompt size |

### Calibration

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `CALIBRATION_ENABLED` | `--calibrate` | `true` | Enable auto-calibration |
| `CALIBRATION_FILE` | `--calibration-file` | `` | Path to persist calibration data |
| `DEFAULT_TOKENS_PER_BYTE` | - | `0.25` | Initial tokens/byte estimate (~4 bytes/token) |
| `DEFAULT_TOKENS_PER_IMAGE` | - | `768` | Tokens per image fallback |

### HTTP/Performance

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `CORS_ALLOW_ORIGIN` | `--cors-origin` | `*` | CORS header (empty to disable) |
| `FLUSH_INTERVAL` | `--flush-interval` | `100ms` | Streaming flush interval |
| `REQUEST_BODY_MAX_BYTES` | `--max-body` | `10MB` | Max request body to parse |
| `RESPONSE_TAP_MAX_BYTES` | `--max-tap` | `5MB` | Max response to buffer for calibration |
| `SHOW_CACHE_TTL` | `--show-ttl` | `5m` | TTL for `/api/show` cache |

---

## Supervisor Features

All supervisor features are **opt-in** and **disabled by default**. Enable the supervisor layer first:

```bash
SUPERVISOR_ENABLED=true
```

When `SUPERVISOR_ENABLED=true`, these monitoring features are **automatically enabled**:
- `SUPERVISOR_TRACK_REQUESTS=true` - Request lifecycle tracking
- `SUPERVISOR_OBS_ENABLED=true` - Observability endpoints (`/debug/requests`, `/events`)
- `SUPERVISOR_METRICS_ENABLED=true` - Prometheus metrics (`/metrics`)
- `SUPERVISOR_HEALTH_CHECK_ENABLED=true` - Upstream health monitoring

You can explicitly disable any of these by setting them to `false` if needed.

### Watchdog Timeouts

Cancels requests that exceed timeout thresholds. Prevents indefinitely hanging requests.

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `SUPERVISOR_WATCHDOG_ENABLED` | `--supervisor-watchdog-enabled` | `false` | Enable watchdog |
| `SUPERVISOR_TTFB_TIMEOUT` | `--supervisor-ttfb-timeout` | `300s` | Time-to-first-byte timeout |
| `SUPERVISOR_STALL_TIMEOUT` | `--supervisor-stall-timeout` | `200s` | Stall timeout (no activity after first byte) |
| `SUPERVISOR_HARD_TIMEOUT` | `--supervisor-hard-timeout` | `12m` | Total request duration limit |

**Recommended values:**
- **Consumer hardware / Local models** (default): `300s` / `200s` / `12m` - Higher timeouts for slower consumer hardware with large contexts
- Interactive chat (enterprise): `30s` / `15s` / `5m`
- Batch processing: `120s` / `60s` / `30m`

### Loop Detection

Detects and cancels degenerate repeating output patterns. Uses n-gram analysis on streaming output.

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `SUPERVISOR_LOOP_DETECT_ENABLED` | `--supervisor-loop-detect` | `false` | Enable loop detection |
| `SUPERVISOR_LOOP_WINDOW_BYTES` | `--supervisor-loop-window` | `4096` | Rolling window size |
| `SUPERVISOR_LOOP_NGRAM_BYTES` | `--supervisor-loop-ngram` | `64` | N-gram size for detection |
| `SUPERVISOR_LOOP_REPEAT_THRESHOLD` | `--supervisor-loop-threshold` | `3` | Repeats needed to trigger |
| `SUPERVISOR_LOOP_MIN_OUTPUT_BYTES` | `--supervisor-loop-min-output` | `1024` | Minimum output before activation |

**Caution:** This is conservative by design. Tune thresholds based on your models.

### Retry Logic

Retries failed requests automatically. **Non-streaming only by default** to preserve streaming semantics.

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `SUPERVISOR_RETRY_ENABLED` | `--supervisor-retry-enabled` | `false` | Enable retries |
| `SUPERVISOR_RETRY_MAX_ATTEMPTS` | `--supervisor-retry-attempts` | `2` | Maximum retry attempts |
| `SUPERVISOR_RETRY_BACKOFF` | `--supervisor-retry-backoff` | `250ms` | Backoff between retries |
| `SUPERVISOR_RETRY_ONLY_NON_STREAMING` | `--supervisor-retry-non-streaming` | `true` | Only retry non-streaming |
| `SUPERVISOR_RETRY_MAX_RESPONSE_BYTES` | `--supervisor-retry-max-response` | `8MB` | Max response to buffer |

**Why non-streaming only?** Retrying after partial streaming output would break the 1:1 API semantics.

### Restart Hook

Restarts Ollama after repeated failures. Protected by cooldown and rate limits.

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `SUPERVISOR_RESTART_ENABLED` | `--supervisor-restart-enabled` | `false` | Enable restart hook |
| `SUPERVISOR_RESTART_CMD` | `--supervisor-restart-cmd` | `` | Command to execute (required) |
| `SUPERVISOR_RESTART_COOLDOWN` | `--supervisor-restart-cooldown` | `120s` | Minimum time between restarts |
| `SUPERVISOR_RESTART_MAX_PER_HOUR` | `--supervisor-restart-max-hour` | `3` | Maximum restarts per hour |
| `SUPERVISOR_RESTART_TRIGGER_CONSEC_TIMEOUTS` | `--supervisor-restart-trigger` | `2` | Consecutive timeouts to trigger |
| `SUPERVISOR_RESTART_CMD_TIMEOUT` | `--supervisor-restart-cmd-timeout` | `30s` | Command execution timeout |

**Example commands:**
```bash
# Systemd
SUPERVISOR_RESTART_CMD="systemctl restart ollama"

# Docker
SUPERVISOR_RESTART_CMD="docker restart ollama"

# Direct
SUPERVISOR_RESTART_CMD="pkill ollama && sleep 2 && ollama serve &"
```

**Warning:** Restarts kill in-flight requests. Use conservative thresholds.

### Observability Endpoints

Real-time monitoring for GUIs and dashboards. **Auto-enabled when `SUPERVISOR_ENABLED=true`**.

| Variable | Flag | Default | Description |
|----------|------|---------|-------------|
| `SUPERVISOR_OBS_ENABLED` | `--supervisor-obs-enabled` | `false` (auto-enabled) | Enable observability |
| `SUPERVISOR_OBS_REQUESTS_ENDPOINT` | `--supervisor-obs-requests` | `true` | Enable `/debug/requests` |
| `SUPERVISOR_OBS_SSE_ENDPOINT` | `--supervisor-obs-sse` | `true` | Enable `/events` SSE |
| `SUPERVISOR_OBS_PROGRESS_INTERVAL` | `--supervisor-obs-progress-interval` | `250ms` | Progress event throttling |
| `SUPERVISOR_RECENT_BUFFER` | `--supervisor-buffer` | `200` | Recent requests to keep |

**Endpoints:**

```bash
# JSON snapshot of current state
curl http://localhost:11435/debug/requests

# Server-Sent Events stream
curl -N http://localhost:11435/events
```

**Event types:** `request_start`, `first_byte`, `progress`, `done`, `canceled`, `timeout_ttfb`, `timeout_stall`, `timeout_hard`, `upstream_error`, `loop_detected`

---

## Example Configuration

### Minimal (just auto-context)

```bash
./ollama-auto-ctx
```

### Production with monitoring

```bash
SUPERVISOR_ENABLED=true \
SUPERVISOR_WATCHDOG_ENABLED=true \
SUPERVISOR_TTFB_TIMEOUT=60s \
SUPERVISOR_STALL_TIMEOUT=30s \
SUPERVISOR_HARD_TIMEOUT=10m \
CALIBRATION_FILE=/var/lib/ollama-auto-ctx/calibration.json \
./ollama-auto-ctx
```

**Note:** `SUPERVISOR_TRACK_REQUESTS`, `SUPERVISOR_OBS_ENABLED`, `SUPERVISOR_METRICS_ENABLED`, and `SUPERVISOR_HEALTH_CHECK_ENABLED` are automatically enabled.

### Aggressive protection

```bash
SUPERVISOR_ENABLED=true \
SUPERVISOR_WATCHDOG_ENABLED=true \
SUPERVISOR_TTFB_TIMEOUT=30s \
SUPERVISOR_LOOP_DETECT_ENABLED=true \
SUPERVISOR_OUTPUT_SAFETY_LIMIT_ENABLED=true \
SUPERVISOR_OUTPUT_SAFETY_LIMIT_TOKENS=10000 \
SUPERVISOR_RETRY_ENABLED=true \
SUPERVISOR_RESTART_ENABLED=true \
SUPERVISOR_RESTART_CMD="systemctl restart ollama" \
./ollama-auto-ctx
```

**Note:** `SUPERVISOR_TRACK_REQUESTS`, `SUPERVISOR_OBS_ENABLED`, `SUPERVISOR_METRICS_ENABLED`, and `SUPERVISOR_HEALTH_CHECK_ENABLED` are automatically enabled.

---

## Thinking Mode Support

Control model thinking/reasoning via system prompt directives:

```json
{
  "model": "qwen3:latest",
  "messages": [
    {"role": "system", "content": "You are helpful. __think=true"}
  ]
}
```

The proxy extracts `__think=<value>`, removes it from the prompt, and injects the appropriate `think` parameter.

| Model Family | Valid Values |
|--------------|--------------|
| qwen3, deepseek | `true`, `false` |
| gpt-oss | `low`, `medium`, `high` |

---

## Health Check

The proxy provides health check endpoints:

```bash
# Basic health (proxy only)
curl http://localhost:11435/healthz
# Returns: ok (200)

# Combined health (proxy + upstream, if health check enabled)
# Returns: ok (200) or upstream unhealthy (503)

# Upstream health details (if health check enabled)
curl http://localhost:11435/healthz/upstream
# Returns: {"healthy": true, "last_check": "2024-01-11T...", "last_error": ""}
```

---

## Architecture

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed flow diagrams and component descriptions.

## Design Decisions

See [docs/DESIGN_DECISIONS.md](docs/DESIGN_DECISIONS.md) for the reasoning behind key choices.

## Contributing

See [docs/AGENTS.md](docs/AGENTS.md) for a quick orientation to the codebase.

---

## License

Apache License 2.0
