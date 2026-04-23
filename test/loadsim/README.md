# loadsim — 200-service OTLP load simulator

A single Go binary that spins up N simulated services as goroutines and drives
sustained OTLP/gRPC traffic into OtelContext. Used to verify backend robustness
and gate releases via `make loadtest`.

## What it does

- Launches `--services` (default 200) concurrent producers, each with its own
  tracer provider and OTLP gRPC exporter.
- Each producer emits `--rate` (default 50) spans/sec for `--duration` (default 60s).
- Spans cycle round-robin across 5 synthetic operations, durations in
  [5ms, 500ms], deterministic 5% error rate.
- Every 10th span is a parent with 1–3 children in the same trace.
- Producers come online linearly over `--warmup` (default 5s).
- Progress reported every 5s; final summary on exit.

## Run

```bash
# Requires OtelContext running on the target endpoint.
make loadtest                           # full 200-service, 60s run
make loadtest-build                     # build-only → bin/loadsim
go test -tags loadtest ./test/loadsim/...  # unit tests
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--endpoint` | `localhost:4317` | OTLP gRPC endpoint |
| `--services` | `200` | Number of simulated services |
| `--rate` | `50` | Spans per second per service |
| `--duration` | `60s` | Test duration |
| `--insecure` | `true` | Skip TLS verification |
| `--tenant-id` | `""` | Attach `x-tenant-id` metadata (empty = omit) |
| `--warmup` | `5s` | Linear producer ramp-up window |

## Output

Progress lines look like `[T+10s] sent=37000 errors=1850 rate=5000/s`,
followed by a summary block with total spans, errors, effective rate.

## What "healthy" looks like

- No OTLP `ResourceExhausted` or `Unavailable` errors in producer output.
- Backend `/ready` returns 200 throughout.
- `/metrics`: `OtelContext_retention_consecutive_failures` stays 0.
- p99 ingestion latency (`otelcontext_http_request_duration_seconds`) stays
  within 2× baseline; goroutine count levels off within 30s.

Caveat: this simulator does **not** start OtelContext — a live backend
must already be accepting gRPC on the target endpoint.
