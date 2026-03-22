# OtelContext UI Redesign — Design Spec

## Goal

Redesign the OtelContext UI to be more intuitive and professional. The service map becomes the hero view, showing stats and flow across all services. Must handle 100-200 services performantly and neatly. Remove low-value pages, consolidate stats into fewer, richer views.

## Navigation (4 Views)

| View | Purpose | Replaces |
|------|---------|----------|
| **Service Map** | Hero view — hierarchical topology with per-service stats, flow edges, side panel detail | Overview, Services, Metrics |
| **Traces** | Trace list + waterfall span detail | (kept, minor cleanup) |
| **Logs** | Live log stream + search + similarity | (kept, minor cleanup) |
| **MCP Console** | JSON-RPC tool surface | (kept as-is) |

### Removed Pages

- **Overview** — KPI stats merged into Service Map top bar
- **Metrics** — traffic/latency/service-map metrics merged into Service Map side panel
- **Archive** — dedicated page removed; archive stats shown in top bar; archive search available via MCP Console tools

## Service Map View (Primary)

### Layout Structure

```
┌─────────────────────────────────────────────────────────────────┐
│ TopNav: Logo │ [Service Map] [Traces] [Logs] [MCP]  │ Stats Bar │
├─────────────────────────────────────────────────────────────────┤
│ Search │ Edge threshold slider │ Zoom controls (+/−/Fit)       │
├──────────────────────────────────────────────┬──────────────────┤
│                                              │   Side Panel     │
│          Hierarchical Service Map            │   (on click)     │
│          (ECharts canvas)                    │                  │
│                                              │  - KPI cards     │
│  GATEWAY tier                                │  - Health score  │
│  API LAYER tier                              │  - Connections   │
│  SERVICES tier                               │  - Recent errors │
│  DATA tier                                   │  - Action links  │
│                                              │                  │
│                              [minimap]       │                  │
└──────────────────────────────────────────────┴──────────────────┘
```

### Top Stats Bar (Global)

Always-visible horizontal strip in the top nav showing:
- Total services (with healthy count)
- Total traces
- Total logs
- Error rate (colored red if >5%)
- DB size (`DBSizeMB` from RepoStats) — serves as storage indicator since archive size is not tracked separately
- WebSocket connection indicator

Data sources: `GET /api/metrics/dashboard` + `GET /api/stats`

**Auto-refresh:** Dashboard stats poll every 30 seconds to keep top bar current.

### Hierarchical Layout (Tier-based)

Services arranged in tiers from top to bottom, representing request flow direction:

| Tier | Criteria | Visual |
|------|----------|--------|
| **Gateway** | Nodes with no inbound edges (entry points) | Top row |
| **API Layer** | Nodes called directly by gateway | Second row |
| **Services** | Internal services (middle of graph) | Third row |
| **Data** | Leaf nodes with no outbound edges (databases, caches) | Bottom row |

**Tier assignment algorithm:**
1. Build adjacency maps (inbound and outbound) from edges
2. BFS from all root nodes (no inbound edges), assigning each node tier = longest path from any root
3. Identify leaf nodes (no outbound edges) — override their tier to max tier (Data layer)
4. Bucket tiers into 4 display rows: tier 0 = Gateway, tier 1 = API Layer, tier 2..max-1 = Services, tier max = Data
5. If a graph has no root nodes (cycles only), fall back to sorting by `span_count` descending and distributing evenly across 4 tiers

**Layout engine:** Compute tier positions in pure JS (BFS), then pass fixed `(x, y)` coordinates to ECharts graph nodes. Layout is computed once on data load (not animated per-frame). No external layout library needed.

### Service Nodes (Compact)

Each node renders as a small card:

```
┌──────────────────┐
│ ● service-name   │
│ 340 rps  0.2% err│
└──────────────────┘
```

- **Health dot**: green (#22c55e), orange (#fb923c), red (#ef4444) based on `status` field
- **Service name**: truncated at ~16 chars, full name on hover tooltip
- **Two metrics**: RPS (from `request_rate_rps`) + error rate (from `error_rate`)
- **Background tint**: subtle color matching health status
- **Border**: colored by health status

Node sizing:
- Normal (< 70 services): ~120x46px
- Dense (70+ services): ~90x36px, smaller font

### Edge Rendering (Progressive Disclosure)

Edges represent service-to-service calls. With 200 services, there could be 500-1000+ edges.

**Default behavior:**
- Show only edges with `call_count` above a configurable threshold
- Default threshold: `max(median call_count, 10)` — ensures meaningful filtering even in sparse graphs where median is very low
- User can adjust via a slider in the toolbar (range: 1 to max call_count)

**Edge visual properties:**
- Width: scaled by `log(call_count)` — range 1px to 4px
- Color: matches health status of the edge (green/yellow/red based on `error_rate`)
- Opacity: higher traffic = more opaque
- Direction: arrows pointing from source to target (top to bottom flow)

**Edge culling for performance:**
- Maximum 500 rendered edges at any time
- Below threshold edges are hidden, not removed from data
- Hovering a node temporarily shows all its edges regardless of threshold

### Side Panel (Service Detail)

Slides in from the right when a service node is clicked. Width: 280px. Contents:

**Header:**
- Health dot + service name + status badge (HEALTHY/DEGRADED/CRITICAL)

**KPI Cards (2x2 grid):**
- RPS (`request_rate_rps`)
- Error Rate (`error_rate`) — colored red if > 5%
- Avg Latency (`avg_latency_ms`)
- P99 Latency (`p99_latency_ms`)

**Health Score Bar:**
- Visual bar 0-1.0 with numeric label
- Color gradient: red → yellow → green

**Connected Services:**
- **Upstream** — services that call this one (inbound edges), with call counts
- **Downstream** — services this one calls (outbound edges), with call counts
- Each entry is clickable → pans the map to that node and opens its side panel (replacing the current one)

**Alerts / Recent Errors:**
- Display the `alerts` array from the `SystemNode` (already available in graph response — e.g., "error rate above 5%", "avg latency above 500ms")
- These are computed server-side during graph generation, no additional API call needed

**Action Links:**
- "View Traces →" — navigates to Traces view, pre-filtered by `service_name`
- "View Logs →" — navigates to Logs view, pre-filtered by `service_name`

### Search

Text input that filters/highlights services by name. As user types:
- Matching nodes get highlighted (bright border/glow)
- Non-matching nodes dim (lower opacity)
- Map pans to center on first match

### Minimap

Small overview in bottom-right corner (~100x80px). Implemented as a second ECharts instance with `silent: true` (no interactions except click-to-navigate), rendering the same graph data at thumbnail scale with simplified nodes (dots only, no labels). A semi-transparent rectangle overlay shows the current viewport position. On click, the main map pans to the corresponding position.

### Zoom Controls

Three buttons in toolbar:
- **+** Zoom in
- **−** Zoom out
- **⊡** Fit all (reset zoom to show entire graph)

ECharts built-in zoom/pan via mouse scroll and drag.

## Traces View

Keep current two-column layout with minor cleanup:
- Left: trace list (25 limit, click to select)
- Right: selected trace waterfall + detail
- Add service filter support (pre-filter when navigating from Service Map)
- Remove JsonViewer at bottom (raw JSON adds clutter, not useful for most users)

## Logs View

Keep current two-column layout with minor cleanup:
- Left: search + severity filter + similarity results
- Right: live log stream (WebSocket)
- Add service filter support (pre-filter when navigating from Service Map)

## MCP Console

Keep as-is. No changes needed — it's already well-designed for its purpose.

## Cross-View Navigation

When clicking "View Traces →" or "View Logs →" from the Service Map side panel, the service name is passed via React state in `App.tsx`:

```typescript
// App.tsx state
const [activeView, setActiveView] = useState<OtelView>('services');
const [serviceFilter, setServiceFilter] = useState<string | null>(null);

// Navigation from side panel
const navigateToTraces = (serviceName: string) => {
  setServiceFilter(serviceName);
  setActiveView('traces');
};
```

`TracesPage` and `LogsPage` receive `serviceFilter` as a prop and use it to pre-filter their initial data fetch. A "clear filter" chip is shown when a filter is active, allowing the user to remove it.

## Data Refresh Strategy

| View | Refresh Mechanism | Interval |
|------|-------------------|----------|
| Top Stats Bar | Polling `useDashboard()` | 30 seconds |
| Service Map | Polling `useSystemGraph()` | 60 seconds (matches backend GraphRAG refresh loop) |
| Logs | WebSocket real-time stream | Continuous |
| Traces | Manual refresh button | On demand |
| MCP Console | On demand | Per tool call |

Service Map refresh preserves current zoom/pan position and selected service. Layout is only recomputed if the node set changes (new/removed services), not on metric updates.

## Technical Implementation

### Dependencies

No new dependencies required. Continue using:
- **ECharts** — canvas rendering for the service map (already installed)
- **Radix UI** — dialogs, tooltips (already installed)
- **Lucide React** — icons (already installed)

Layout is computed in pure JS via BFS tier assignment — no external layout library needed.

### Performance Strategy (100-200 Services)

| Technique | Purpose |
|-----------|---------|
| ECharts canvas renderer | All nodes/edges rendered to single canvas, not DOM |
| Tier layout computed once | No per-frame physics simulation (BFS runs once on data load) |
| Edge threshold culling | Max 500 visible edges |
| Semantic zoom | At zoom < 0.6: hide metric text on nodes, show only health dot + name. At zoom < 0.3: collapse nodes to colored dots only. Implemented via ECharts `graphScaleLimit` + zoom event listener toggling label visibility |
| Dense mode auto-detection | Smaller nodes when count > 70 |
| `React.memo` on side panel | Prevent re-render on map pan/zoom |
| Debounced search | Filter during typing, not on every keystroke |

### Data Flow

```
App.tsx
├── useSystemGraph()     → Service Map nodes + edges
├── useDashboard()       → Top bar stats (traces, logs, errors, services)
├── useTraces()          → Traces view
├── useLogs()            → Logs view
├── useWebSocket()       → Real-time log stream
└── useMCP()             → MCP Console
```

The `useMetrics()` hook can be simplified or removed — the Service Map gets its data from `useSystemGraph()` which already includes per-service metrics and edge stats.

### Files to Modify

| File | Change |
|------|--------|
| `App.tsx` | Remove Overview/Metrics/Archive views, update nav |
| `TopNav.tsx` | 4 nav items instead of 7, add global stats bar |
| `ServicesPage.tsx` | Complete rewrite — hierarchical layout, side panel, search, edge threshold |
| `TracesPage.tsx` | Add service filter prop, remove JsonViewer |
| `LogsPage.tsx` | Add service filter prop |
| `useSystemGraph.ts` | Add tier computation logic |
| `types/api.ts` | Add `db_size_mb` to `RepoStats` interface, remove unused types |

### Files to Delete

| File | Reason |
|------|--------|
| `OverviewPage.tsx` | Merged into Service Map + top bar |
| `MetricsPage.tsx` | Merged into Service Map side panel |
| `ArchivePage.tsx` | Stats in top bar, search via MCP |
| `useArchive.ts` | No longer needed |
| `useMetrics.ts` | No longer needed (data comes from useSystemGraph) |
| `JsonViewer.tsx` | No remaining consumers after removing OverviewPage and TracesPage usage |

### Color System

Health status colors (consistent across nodes, edges, badges):
- **Healthy**: `#22c55e` (green) — background tint `#0f2618`, border `#166534`
- **Degraded**: `#fb923c` (orange) — background tint `#1a1207`, border `#854d0e` (matches existing codebase convention)
- **Critical**: `#ef4444` (red) — background tint `#1c0707`, border `#991b1b`
- **Data stores**: `#818cf8` (indigo) — background tint `#0c0c1f`, border `#312e81`

### Tier Label Styling

Vertical text labels on the left edge of the map canvas:
- Font: 9px uppercase
- Color: `#3f3f46` (subtle zinc)
- Labels: GATEWAY, API LAYER, SERVICES, DATA
