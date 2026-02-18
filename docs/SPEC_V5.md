# ARGUS V5.0 — Production Hardened Edition

> **Codename**: ARGUS V5.0  
> **Vibe**: Competence, Reliability, Speed  
> **Architecture**: Single-Binary Go Service (Backend + Embedded React UI)  
> **Goal**: Zero-Code OTel Collector — Resilient, Self-Monitoring, Open Access  
> **Timezone**: All backend timestamps are UTC (enforced via `time.Local = time.UTC` at startup)

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
| **gRPC Compression** | `google.golang.org/grpc/encoding/gzip` | Built-in |
| **Frontend** | React 18, TypeScript, Vite | — |
| **UI Framework** | Mantine v7 (Light Mode) + Mantine Notifications | v7.x |
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
│  │  + Severity/Service     │                             │
│  │    Filtering Engine     │                             │
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
    main.go              # Entry point, init chain, gRPC+HTTP servers, UTC enforcement

internal/
  api/
    handlers.go          # HTTP API routes + admin endpoints + service metadata
    handlers_v2.go       # Dashboard metrics, log filtering, service map
  ai/
    service.go           # Optional AI analysis (Azure OpenAI via LangChainGo)
  config/
    config.go            # Env-based configuration (includes ingestion filtering)
  ingest/
    otlp.go              # OTLP gRPC trace + log handlers + severity/service filtering
  middleware/             # Reserved for future middleware (CORS, auth, etc.)
  queue/
    dlq.go               # Dead Letter Queue + Replay Worker
  realtime/
    hub.go               # Buffered WebSocket Hub
  storage/
    models.go            # GORM models (Trace, Span, Log)
    factory.go           # Universal DB Factory (SQLite/PG/MySQL/MSSQL)
    repository.go        # Core CRUD operations
    repository_v2.go     # Advanced queries (traffic, heatmap, dashboard, service-map)
  telemetry/
    metrics.go           # Prometheus self-monitoring

web/
  src/
    api/                 # API client functions (fetch wrappers)
    assets/              # Static assets (SVG, images)
    components/          # Shared UI components
    features/
      dashboard/         # ECharts dashboard (Traffic, Latency Heatmap, Top Failing)
      logs/
        LogExplorer.tsx  # Live log stream (WS + TanStack Virtual + inline expansion)
      topology/          # Force-directed Service Map (ECharts graph)
      settings/          # Health stats + Danger Zone (purge/vacuum)
      traces/            # Trace explorer + Span Drill-Down
    hooks/               # Custom React hooks
    layouts/             # Mantine AppShell + Header
    lib/                 # Utility functions
    styles/              # Global CSS styles
    env.d.ts             # Vite environment type declarations
    theme.ts             # Mantine theme config (Inter font, indigo primary, md radius)
    types.ts             # Shared TypeScript interfaces
    main.tsx             # MantineProvider + Notifications + QueryClient entry point

test/
  orderservice/          # Service A (Port 9001)
  paymentservice/        # Service B (Port 9002)
  inventoryservice/      # Service C (Port 9003)
  run_simulation.ps1     # PowerShell lifecycle orchestrator
  run_simulation.sh      # Bash lifecycle orchestrator (cross-platform)
  load_test.ps1          # Standalone load test script
  scripts/
    check_links/         # Go utility: validate service connectivity
    dump_spans/          # Go utility: dump raw span data for debugging
    truncate_db/         # Go utility: truncate database tables

.github/
  workflows/
    audit.yml            # Security audit (govulncheck on push/PR/weekly)
    release.yml          # Manual release pipeline (build UI → tag → GitHub Release)
```

---

## Phase 1: Go Backend Core

### 1.1 UTC Timezone Enforcement

**Problem**: Backend timestamps reflect the host machine's local timezone, causing inconsistency across deployments.

**Solution**: `time.Local = time.UTC` set as the very first line in `main()`, ensuring all `time.Now()` calls across the entire process return UTC.

### 1.2 Ingestion Filtering Engine (`internal/ingest/otlp.go`)

**Problem**: High-volume environments may send noise (debug logs, internal services) that wastes storage.

**Solution**: Configurable ingestion-time filtering on both the `TraceServer` and `LogsServer`:
- **Severity Filtering**: `INGEST_MIN_SEVERITY` (default: `INFO`) — drops logs below threshold (DEBUG=10, INFO=20, WARN=30, ERROR=40, FATAL=50)
- **Service Allow List**: `INGEST_ALLOWED_SERVICES` — comma-separated; only ingest from listed services (empty = allow all)
- **Service Deny List**: `INGEST_EXCLUDED_SERVICES` — comma-separated; drop traces/logs from listed services
- Deny list is evaluated before allow list

### 1.3 Resilience Engine — Dead Letter Queue (`internal/queue/dlq.go`)

**Problem**: If the database is down, ingested telemetry data is lost.

**Solution**:
- On `db.CreateInBatches` failure → log error → serialize batch to JSON → write to `./data/dlq/batch_{timestamp}.json`
- **Replay Worker**: Background goroutine checks DLQ directory at configurable interval (default: 5 minutes)
- On success: delete the replayed file
- `DLQSize() int` method exposes count for Prometheus gauge
- **Configurable**: `DLQ_PATH` and `DLQ_REPLAY_INTERVAL` environment variables

### 1.4 Buffered WebSocket Hub (`internal/realtime/hub.go`)

**Problem**: Broadcasting 5,000 logs/sec individually will freeze the frontend.

**Solution**: Buffered broadcast with flush triggers.
- Buffer: `[]storage.Log` protected by `sync.Mutex`
- **Flush when**: `len(buffer) >= 100` items **OR** 500ms ticker fires
- Sends JSON `[]LogEntry` array per WebSocket message
- Library: `github.com/coder/websocket` v1.8.14 (zero CVEs)
- Client lifecycle managed via register/unregister channels

### 1.5 Self-Monitoring Telemetry (`internal/telemetry/metrics.go`)

| Metric | Type | Description |
|--------|------|-------------|
| `argus_ingestion_rate` | Counter | Total spans/logs ingested |
| `argus_active_connections` | Gauge | Current WebSocket clients |
| `argus_db_latency` | Histogram | DB operation duration (seconds) |
| `argus_dlq_size` | Gauge | Files in DLQ directory |

- Exposed at `GET /metrics` (Prometheus format)
- Also exposed at `GET /api/health` (JSON for frontend)

### 1.6 Universal Storage Factory

Supported drivers: `sqlite`, `postgres`, `mysql`, `sqlserver`

Optimizations:
- **SQLite**: WAL mode, busy_timeout=5000, synchronous=NORMAL, MaxOpen=1
- **Others**: Connection pooling (MaxOpen=50, MaxIdle=10, MaxLifetime=1h)
- **Postgres**: Added via `gorm.io/driver/postgres`
- **MySQL**: Uses `INSERT IGNORE` to avoid Error 1869 (auto-increment conflict)

### 1.7 gRPC

- Supports gzip compression via `google.golang.org/grpc/encoding/gzip` (auto-registered)
- gRPC reflection enabled for debugging with tools like `grpcurl`

### 1.8 Security

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

### Lifecycle Scripts
- **`test/run_simulation.ps1`** (PowerShell): Build → Start → 300 requests → Summary → Cleanup
- **`test/run_simulation.sh`** (Bash): Cross-platform alternative
- **`test/load_test.ps1`**: Standalone load test (no build/cleanup, just HTTP requests)

### Utility Scripts (`test/scripts/`)
- **`check_links/`**: Validate connectivity between test services
- **`dump_spans/`**: Dump raw span data from the database for debugging
- **`truncate_db/`**: Truncate all telemetry tables for a clean test run

---

## Phase 3: React Frontend

### Visual Identity
- **Theme**: Mantine v7, Light Mode default, Indigo primary color
- **Font**: Inter (via `theme.ts`)
- **Style**: Modern SaaS with `md` border radius
- **Charts**: Apache ECharts (replaces Highcharts)
- **Notifications**: Mantine Notifications (top-right position)

### Pages

| Page | Route Key | Components |
|------|-----------|------------|
| Dashboard | `dashboard` | ECharts Traffic + Latency Heatmap + Top Failing Services, Smart Filter Bar (dynamic service list) |
| Service Map | `map` | Force-directed ECharts graph (nodes=services/metrics, edges=trace connections/errors) + Navigation |
| Logs | `logs` | TanStack Virtual + WebSocket live + Inline Expansion + ±1min Context |
| Traces | `traces` | Trace explorer + Span Drill-Down + Link to Logs |
| Settings | `settings` | Argus Health (live metrics) + Danger Zone (purge/vacuum) |

### Dynamic Service Filters
- All filter dropdowns (Dashboard, Traces, Logs) fetch service names dynamically from `GET /api/metadata/services`
- No hardcoded service names in the frontend

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

### Dynamic Versioning
- **Local dev**: UI displays `DEV`
- **Release builds**: `APP_VERSION` env var injected during CI build, displayed in header
- Configured via Vite's `import.meta.env.VITE_APP_VERSION`

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
```

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

### TypeScript Interfaces
Shared type definitions in `web/src/types.ts`:
- `Trace`, `Span`, `LogEntry`, `LogResponse`, `TraceResponse`
- `TrafficPoint`, `LatencyPoint`, `ServiceError`
- `DashboardStats`, `HealthStats`
- `ServiceMapNode`, `ServiceMapEdge`, `ServiceMapMetrics`

---

## Ports & Endpoints

| Port | Protocol | Service |
|------|----------|---------|
| 4317 | gRPC | OTLP Trace + Log Receiver (gzip supported) |
| 8080 | HTTP | API + Embedded UI + Metrics |
| 9001 | HTTP | Order Service (test) |
| 9002 | HTTP | Payment Service (test) |
| 9003 | HTTP | Inventory Service (test) |

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/stats` | Basic trace/error counts |
| GET | `/api/traces` | Filtered traces (pagination, time range, service, status, search, sort) |
| GET | `/api/logs` | Filtered logs (pagination, time range, service, severity, search) |
| GET | `/api/logs/context` | Context logs (±1 min around timestamp) |
| GET | `/api/logs/{id}/insight` | AI insight for log |
| GET | `/api/metrics/traffic` | Traffic chart data (time range, service filter) |
| GET | `/api/metrics/latency_heatmap` | Latency heatmap data (time range, service filter) |
| GET | `/api/metrics/dashboard` | Dashboard aggregates (time range, service filter) |
| GET | `/api/metrics/service-map` | Service Map nodes & edges with metrics |
| GET | `/api/metadata/services` | Distinct service names (for dynamic filters) |
| GET | `/api/health` | Self-monitoring stats (JSON) |
| GET | `/metrics` | Prometheus metrics |
| GET | `/ws` | WebSocket log stream |
| DELETE | `/api/admin/purge` | Purge old logs (default: 7 days, configurable via `?days=N`) |
| POST | `/api/admin/vacuum` | Vacuum database |

---

## CI/CD

### Security Audit (`audit.yml`)
- **Trigger**: Push to main, pull requests, weekly schedule
- **Action**: Runs `govulncheck ./...` to detect known Go vulnerabilities

### Release Pipeline (`release.yml`)
- **Trigger**: Manual (`workflow_dispatch`) with version input (e.g., `v1.0.0`)
- **Steps**:
  1. Checkout code
  2. Setup Node.js 20
  3. Build frontend (`npm ci && npm run build`) with `APP_VERSION` env var
  4. Verify `web/dist` exists
  5. Commit built UI assets
  6. Create Git tag
  7. Create GitHub Release with auto-generated release notes

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_ENV` | `development` | Environment name |
| `LOG_LEVEL` | `INFO` | Logging level (DEBUG, INFO, WARN, ERROR) |
| `HTTP_PORT` | `8080` | HTTP server port |
| `GRPC_PORT` | `4317` | gRPC server port |
| `DB_DRIVER` | `sqlite` | Database driver (sqlite, postgres, mysql, sqlserver) |
| `DB_DSN` | *(empty)* | Database connection string |
| `DLQ_PATH` | `./data/dlq` | Directory for Dead Letter Queue files |
| `DLQ_REPLAY_INTERVAL` | `5m` | How often DLQ replays failed batches (Go duration) |
| `INGEST_MIN_SEVERITY` | `INFO` | Minimum severity to ingest (DEBUG, INFO, WARN, ERROR, FATAL) |
| `INGEST_ALLOWED_SERVICES` | *(empty)* | Comma-separated allow list (empty = allow all) |
| `INGEST_EXCLUDED_SERVICES` | *(empty)* | Comma-separated deny list |
| `AI_ENABLED` | `false` | Enable AI analysis |
| `AZURE_OPENAI_*` | — | Azure OpenAI configuration |
