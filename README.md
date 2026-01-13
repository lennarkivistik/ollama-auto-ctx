# ollama-auto-ctx

An intelligent context-sizing proxy for [Ollama](https://ollama.com) that automatically calculates the optimal `num_ctx` for each request, with built-in dashboard, telemetry, and reliability features.

## Quick Start

```bash
# Run with defaults (MODE=retry, SQLite storage, dashboard enabled)
./ollama-auto-ctx

# Point your Ollama clients to :11435 instead of :11434
export OLLAMA_HOST=http://localhost:11435
```

Open the dashboard at [http://localhost:11435/dashboard](http://localhost:11435/dashboard).

## How It Works

For every `/api/chat` or `/api/generate` request:

1. **Estimates** prompt tokens from message content, tools, and images
2. **Calculates** the required context: `prompt_tokens + output_budget * headroom`
3. **Snaps** to the nearest bucket size (e.g., 4096, 8192, 16384...)
4. **Injects** `options.num_ctx` into the request
5. **Forwards** to Ollama and streams the response back

## Modes

The `MODE` environment variable controls which features are enabled:

| Mode | Dashboard | API | Metrics | SQLite | Retry | Protect |
|------|-----------|-----|---------|--------|-------|---------|
| `off` | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| `monitor` | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| `retry` (default) | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ |
| `protect` | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

- **off**: Pure proxy with context sizing only. No dashboard, no storage.
- **monitor**: Full observability without retry/protect. Good for debugging.
- **retry** (default): Adds automatic retry with exponential backoff for failed requests.
- **protect**: Adds watchdog timeouts, loop detection, and output limits.

## Dashboard

When `MODE != off`, the dashboard is available at `/dashboard` with:

- **Summary cards** with sparkline graphs
- **Request table** with click-to-expand details
- **Request modal** with Overview/Timings/Tokens/Bytes tabs
- **Real-time updates** via API polling

## SQLite Storage

Request telemetry is stored in SQLite (default `/data/oac.sqlite`) with:

- **WAL mode** for concurrent performance
- **Row-capped retention** (default 3000 rows)
- **Metadata only** - no prompt/response content stored

## API

All API endpoints are under `/autoctx/api/v1/`:

| Endpoint | Description |
|----------|-------------|
| `GET /overview?window=1h\|24h\|7d` | Summary stats + time series |
| `GET /requests?limit=50&offset=0` | Paginated request list |
| `GET /requests/{id}` | Single request details |
| `GET /models` | Per-model statistics |
| `GET /models/{model}/series` | Model sparkline data |
| `GET /config` | Current configuration |

## Prometheus Metrics

When `MODE != off`, Prometheus metrics are available at `/metrics`:

```
oac_requests_total{model, status, reason}
oac_retries_total{model}
oac_request_duration_seconds{model}
oac_ttfb_seconds{model}
oac_requests_in_flight
oac_upstream_healthy
```

## Configuration

All configuration is via environment variables:

### Core

| Variable | Default | Description |
|----------|---------|-------------|
| `MODE` | `retry` | off / monitor / retry / protect |
| `LISTEN_ADDR` | `:11435` | Proxy listen address |
| `UPSTREAM_URL` | `http://127.0.0.1:11434` | Ollama server URL |
| `LOG_LEVEL` | `info` | debug / info / warn / error |

### Storage

| Variable | Default | Description |
|----------|---------|-------------|
| `STORAGE` | `sqlite` | sqlite / memory / off (auto-falls back to memory on unsupported platforms) |
| `STORAGE_PATH` | `/data/oac.sqlite` | SQLite database file path |
| `STORAGE_MAX_ROWS` | `3000` | Maximum rows before pruning |

### Retry (MODE=retry or protect)

| Variable | Default | Description |
|----------|---------|-------------|
| `RETRY_MAX` | `2` | Maximum retry attempts |
| `RETRY_BACKOFF_MS` | `1000` | Backoff between retries (ms) |

### Protect (MODE=protect only)

| Variable | Default | Description |
|----------|---------|-------------|
| `TIMEOUT_TTFB_MS` | `15000` | Time to first byte timeout |
| `TIMEOUT_STALL_MS` | `30000` | Stall detection timeout |
| `TIMEOUT_HARD_MS` | `300000` | Hard request timeout |
| `LOOP_DETECT_ENABLED` | `true` | Enable loop detection |
| `OUTPUT_LIMIT_ENABLED` | `true` | Enable output token limit |
| `OUTPUT_LIMIT_MAX_TOKENS` | `4096` | Maximum output tokens |

### Context Sizing

| Variable | Default | Description |
|----------|---------|-------------|
| `MIN_CTX` | `1024` | Minimum context size |
| `MAX_CTX` | `81920` | Maximum context size |
| `BUCKETS` | `1024,2048,4096,...` | Context bucket sizes |
| `HEADROOM` | `1.25` | Headroom multiplier (1.25 = 25%) |
| `DEFAULT_OUTPUT_BUDGET` | `1024` | Default output token budget |
| `MAX_OUTPUT_BUDGET` | `10240` | Maximum output budget |
| `CALIBRATION_ENABLED` | `true` | Enable model calibration |

## Docker

```dockerfile
FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o ollama-auto-ctx ./cmd/ollama-auto-ctx

FROM alpine:latest
COPY --from=builder /app/ollama-auto-ctx /ollama-auto-ctx
EXPOSE 11435
ENTRYPOINT ["/ollama-auto-ctx"]
```

```bash
docker build -t ollama-auto-ctx .
docker run -v /data:/data -e UPSTREAM_URL=http://host.docker.internal:11434 ollama-auto-ctx
```

## Health Checks

| Endpoint | Description |
|----------|-------------|
| `GET /healthz` | Proxy health (includes upstream if enabled) |
| `GET /healthz/upstream` | Detailed upstream health JSON |

## Response Headers

The proxy adds these headers to responses:

| Header | Description |
|--------|-------------|
| `X-Ollama-CtxProxy-Clamped` | Present if context was clamped to model/config max |

## Architecture

```
Client → Proxy (:11435) → Ollama (:11434)
              ↓
         Dashboard
         SQLite DB
         Metrics
```

The proxy is designed to be transparent to clients - it just adds the `options.num_ctx` field to requests and optionally tracks telemetry.

## License

MIT
