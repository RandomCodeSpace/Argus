# ARGUS V5.0 — Production Hardened Edition (DEV MODE)

> **Codename**: ARGUS V5.0  
> **Vibe**: Competence, Reliability, Speed  
> **Architecture**: Single-Binary Go Service (Backend + Embedded React UI)  
> **Goal**: Zero-Code OTel Collector — Resilient, Self-Monitoring, Open Access

---

## Technology Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| **Language** | Go | 1.24+ |
| **ORM** | GORM | Latest |
| **DB Support** | SQLite, PostgreSQL, MySQL, SQL Server | Universal Factory |
| **WebSocket** | `github.com/coder/websocket` | v1.8.14 (zero CVEs) |
| **Metrics** | `prometheus/client_golang` | Latest |
| **gRPC** | `google.golang.org/grpc` | v1.79+ |
| **Frontend** | React 18, TypeScript, Vite | — |
| **UI Framework** | Mantine v7 (Light Mode) | v7.x |
| **Charts** | Apache ECharts (`echarts-for-react`) | v5.x |
| **State** | TanStack Query, TanStack Table | v5, v8 |
| **Virtualization** | TanStack Virtual | v3 |
| **AI (Optional)** | LangChainGo + Azure OpenAI | Preserved |

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     ARGUS V5.0 Binary                   │
│                                                         │
│  ┌──────────┐  ┌───────────┐  ┌──────────────────────┐  │
│  │  gRPC    │  │  HTTP API  │  │  Embedded React UI  │  │
│  │  :4317   │  │  :8080     │  │  (Mantine + ECharts) │  │
│  └────┬─────┘  └─────┬─────┘  └──────────────────────┘  │
│       │              │                                   │
│  ┌────▼──────────────▼─────┐                             │
│  │     OTLP Ingest Layer   │                             │
│  │  (Traces + Logs gRPC)   │                             │
│  └────────────┬────────────┘                             │
│               │                                          │
│  ┌────────────▼────────────┐  ┌────────────────────┐     │
│  │   Async Batcher + DLQ   │  │  Buffered WS Hub   │     │
│  │   internal/queue        │  │  internal/realtime  │     │
│  └────────────┬────────────┘  └────────┬───────────┘     │
│               │                        │                 │
│  ┌────────────▼────────────┐  ┌────────▼───────────┐     │
│  │   GORM Storage Layer    │  │  Prometheus Metrics │     │
│  │   SQLite/PG/MySQL/MSSQL │  │  internal/telemetry │     │
│  └─────────────────────────┘  └────────────────────┘     │
└─────────────────────────────────────────────────────────┘
```

---

## Project Structure

```
cmd/
  argus/
    main.go              # Entry point, init chain, gRPC+HTTP servers

internal/
  api/
    handlers.go          # HTTP API routes + admin endpoints
    handlers_v2.go       # Dashboard metrics, log filtering
  ai/
    service.go           # Optional AI analysis (Azure OpenAI)
  config/
    config.go            # Env-based configuration
  ingest/
    otlp.go              # OTLP gRPC trace + log handlers
  queue/
    dlq.go               # [NEW] Dead Letter Queue + Replay Worker
  realtime/
    hub.go               # [NEW] Buffered WebSocket Hub
  storage/
    models.go            # GORM models (Trace, Span, Log)
    factory.go           # [NEW] Universal DB Factory
    repository.go        # Core CRUD operations
    repository_v2.go     # Advanced queries (traffic, heatmap, etc.)
  telemetry/
    metrics.go           # [NEW] Prometheus self-monitoring

web/
  src/
    features/
      dashboard/         # ECharts dashboard (Traffic, Latency, Top Failing)
      logs/              # Live log stream (WS + TanStack Virtual)
      topology/          # [NEW] Force-directed Service Map
      settings/          # Health stats + Danger Zone
      traces/            # Trace explorer
    layouts/             # Mantine AppShell + Header
    main.tsx             # MantineProvider entry point

test/
  orderservice/          # Service A (Port 9001)
  paymentservice/        # Service B (Port 9002)
  inventoryservice/      # [NEW] Service C (Port 9003)
  run_simulation.ps1     # [NEW] PowerShell lifecycle orchestrator
```

---

## Phase 1: Go Backend Core

### 1.1 Resilience Engine — Dead Letter Queue (`internal/queue/dlq.go`)

**Problem**: If the database is down, ingested telemetry data is lost.

**Solution**:
- On `db.CreateInBatches` failure → log error → serialize batch to JSON → write to `./data/dlq/batch_{timestamp}.json`
- **Replay Worker**: Background goroutine checks `./data/dlq/` every 5 minutes, re-inserts files
- On success: delete the replayed file
- `DLQSize() int` method exposes count for Prometheus gauge

### 1.2 Buffered WebSocket Hub (`internal/realtime/hub.go`)

**Problem**: Broadcasting 5,000 logs/sec individually will freeze the frontend.

**Solution**: Buffered broadcast with flush triggers.
- Buffer: `[]storage.Log` protected by `sync.Mutex`
- **Flush when**: `len(buffer) >= 100` items **OR** 500ms ticker fires
- Sends JSON `[]LogEntry` array per WebSocket message
- Library: `github.com/coder/websocket` v1.8.14 (zero CVEs)
- Client lifecycle managed via register/unregister channels

### 1.3 Self-Monitoring Telemetry (`internal/telemetry/metrics.go`)

| Metric | Type | Description |
|--------|------|-------------|
| `argus_ingestion_rate` | Counter | Total spans/logs ingested |
| `argus_active_connections` | Gauge | Current WebSocket clients |
| `argus_db_latency` | Histogram | DB operation duration (seconds) |
| `argus_dlq_size` | Gauge | Files in DLQ directory |

- Exposed at `GET /metrics` (Prometheus format)
- Also exposed at `GET /api/health` (JSON for frontend)

### 1.4 Universal Storage Factory

Supported drivers: `sqlite`, `postgres`, `mysql`, `sqlserver`

Optimizations:
- **SQLite**: WAL mode, busy_timeout=5000, synchronous=NORMAL, MaxOpen=1
- **Others**: Connection pooling (MaxOpen=50, MaxIdle=10, MaxLifetime=1h)
- **Postgres**: Added via `gorm.io/driver/postgres`
- **MySQL**: Uses `INSERT IGNORE` to avoid Error 1869 (auto-increment conflict)

### 1.5 Security

- GitHub Actions `audit.yml` workflow runs `govulncheck ./...` on push/PR/weekly schedule
- All WebSocket connections use `github.com/coder/websocket` (zero known CVEs)
- SQL injection prevention via GORM parameterized queries + sort field whitelisting

---

## Phase 2: Chaos Test Services

Three lightweight Go microservices to stress-test Argus with complex distributed traces.

### Service A — Order Service (Port 9001)
- **Behavior**: Start trace → Call Service B → Chaos (30% random latency 100-800ms)

### Service B — Payment Service (Port 9002)
- **Behavior**: Accept context → Call Service C → Chaos (10% HTTP 500 "Gateway Timeout")

### Service C — Inventory Service (Port 9003)
- **Behavior**: Accept context → Chaos (5% "Database Lock" — 2-5s latency + error)

### Lifecycle Script (`test/run_simulation.ps1`)
1. Build all 3 services to `tmp/`
2. Start minimized, capture process objects
3. Run 300 POST requests to Service A (200ms intervals, ~60s)
4. Print summary (total/success/fail/error rate)
5. Cleanup: kill processes + delete binaries

---

## Phase 3: React Frontend

### Visual Identity
- **Theme**: Mantine v7, Light Mode default
- **Style**: Modern SaaS
- **Charts**: Apache ECharts (replaces Highcharts)

### Pages

| Page | Route Key | Components |
|------|-----------|------------|
| Dashboard | `dashboard` | ECharts Traffic + Latency Heatmap + Top Failing Services, Smart Filter Bar |
| Service Map | `map` | Force-directed ECharts graph (nodes=services/metrics, edges=trace connections/errors) + Navigation |
| Logs | `logs` | TanStack Virtual + WebSocket live + Inline Expansion + ±1min Context |
| Traces | `traces` | Trace explorer + Span Drill-Down + Link to Logs |
| Settings | `settings` | Argus Health (live metrics) + Danger Zone (purge/vacuum) |

### Default Time Ranges
- **All screens default to 5 minutes** of historical data
- Dashboard includes time range selector (5m, 15m, 1h, 6h, 24h)
- Auto-refresh every 10–30 seconds per screen

### WebSocket Log Streaming
- **Off by default** — historical logs fetched via API (last 5 min, 10s refresh)
- Toggle to **Live mode** to connect to `ws://host/ws`
- Ref-based connection tracking prevents duplicate WebSocket connections
- Receives `Log[]` arrays (batched by hub, max 100 items or 500ms flush)
- Appended to local state, virtualized rendering via TanStack Virtual (caps at 2000 rows)

### Dashboard Stats API Response
```json
{
  "total_traces": 301,
  "total_logs": 42,
  "total_errors": 15,
  "avg_latency_ms": 245.3,
  "error_rate": 4.98,
  "active_services": 3,
  "p99_latency": 850000,
  "top_failing_services": [
    { "service_name": "payment-service", "error_count": 10, "total_count": 100, "error_rate": 0.1 }
  ]
}

### Settings — Self-Monitoring
Fetches from `GET /api/health`:
```json
{
  "ingestion_rate": 12345,
  "dlq_size": 0,
  "active_connections": 3,
  "db_latency_p99_ms": 12.5
}
```

---

## Ports & Endpoints

| Port | Protocol | Service |
|------|----------|---------|
| 4317 | gRPC | OTLP Trace + Log Receiver |
| 8080 | HTTP | API + Embedded UI + Metrics |
| 9001 | HTTP | Order Service (test) |
| 9002 | HTTP | Payment Service (test) |
| 9003 | HTTP | Inventory Service (test) |

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/stats` | Basic trace/error counts |
| GET | `/api/traces` | Filtered traces (pagination) |
| GET | `/api/logs` | Filtered logs (pagination) |
| GET | `/api/logs/context` | Context logs (±1 min) |
| GET | `/api/logs/{id}/insight` | AI insight for log |
| GET | `/api/metrics/traffic` | Traffic chart data |
| GET | `/api/metrics/latency_heatmap` | Latency heatmap data |
| GET | `/api/metrics/dashboard` | Dashboard aggregates |
| GET | `/api/metrics/service-map` | Service Map nodes & edges with metrics |
| GET | `/api/metadata/services` | Distinct service names |
| GET | `/api/health` | Self-monitoring stats (JSON) |
| GET | `/metrics` | Prometheus metrics |
| GET | `/ws` | WebSocket log stream |
| DELETE | `/api/admin/purge` | Purge old logs |
| POST | `/api/admin/vacuum` | Vacuum database |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_ENV` | `development` | Environment name |
| `LOG_LEVEL` | `INFO` | Logging level |
| `HTTP_PORT` | `8080` | HTTP server port |
| `GRPC_PORT` | `4317` | gRPC server port |
| `DB_DRIVER` | `sqlite` | Database driver |
| `DB_DSN` | `argus.db` | Database connection string |
| `AI_ENABLED` | `false` | Enable AI analysis |
| `AZURE_OPENAI_*` | — | Azure OpenAI configuration |
