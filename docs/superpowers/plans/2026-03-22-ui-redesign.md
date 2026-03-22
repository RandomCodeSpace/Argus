# UI Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate the OtelContext UI from 7 pages to 4, with a premium hierarchical service map as the hero view, handling 100-200 services performantly.

**Architecture:** Replace force-directed graph with tier-based hierarchical layout computed via BFS. Service Map becomes the default view with inline stats, side panel on click, progressive edge disclosure. Cross-view navigation passes service filter via React state. Vitest + Testing Library for tests.

**Tech Stack:** React 19, ECharts 6 (canvas), Radix UI, Lucide React, Vitest, @testing-library/react

**Spec:** `docs/superpowers/specs/2026-03-22-ui-redesign-design.md`

---

## File Structure

### New Files
| File | Purpose |
|------|---------|
| `ui/vitest.config.ts` | Vitest configuration for React/TypeScript |
| `ui/src/lib/tierLayout.ts` | BFS tier assignment + coordinate computation (pure logic, no React) |
| `ui/src/lib/__tests__/tierLayout.test.ts` | Tests for tier layout algorithm |
| `ui/src/components/observability/ServiceMap.tsx` | Complete service map rewrite (ECharts graph + toolbar + minimap) |
| `ui/src/components/observability/ServiceSidePanel.tsx` | Side panel component (KPIs, connections, alerts, actions) |
| `ui/src/components/observability/__tests__/ServiceSidePanel.test.tsx` | Tests for side panel rendering |
| `ui/src/hooks/__tests__/useSystemGraph.test.ts` | Tests for graph hook with polling |

### Modified Files
| File | Changes |
|------|---------|
| `ui/package.json` | Add vitest, @testing-library/react, @testing-library/jest-dom, jsdom |
| `ui/src/types/api.ts` | Add `db_size_mb` to RepoStats, remove unused types |
| `ui/src/App.tsx` | Remove 3 views, add serviceFilter state, cross-view nav |
| `ui/src/components/nav/TopNav.tsx` | 4 nav items, global stats bar |
| `ui/src/hooks/useSystemGraph.ts` | Add polling interval, tier computation integration |
| `ui/src/hooks/useDashboard.ts` | Add polling interval (30s) |
| `ui/src/components/observability/TracesPage.tsx` | Add serviceFilter prop, remove JsonViewer |
| `ui/src/components/observability/LogsPage.tsx` | Add serviceFilter prop |
| `ui/src/styles/global.css` | Add service-map, side-panel, stats-bar CSS classes |

### Deleted Files
| File | Reason |
|------|--------|
| `ui/src/components/observability/OverviewPage.tsx` | Merged into Service Map + top bar |
| `ui/src/components/observability/MetricsPage.tsx` | Merged into Service Map side panel |
| `ui/src/components/observability/ArchivePage.tsx` | Stats in top bar, search via MCP |
| `ui/src/hooks/useArchive.ts` | No longer needed |
| `ui/src/hooks/useMetrics.ts` | No longer needed |
| `ui/src/components/shared/JsonViewer.tsx` | No remaining consumers |
| `ui/src/components/observability/ServicesPage.tsx` | Replaced by ServiceMap.tsx |

---

## Task 1: Set Up Vitest Test Infrastructure

**Files:**
- Modify: `ui/package.json`
- Create: `ui/vitest.config.ts`
- Create: `ui/src/test-setup.ts`

- [ ] **Step 1: Install test dependencies**

```bash
cd /home/dev/git/otelcontext/ui && npm install --save-dev vitest @testing-library/react @testing-library/jest-dom @testing-library/user-event jsdom @types/node
```

- [ ] **Step 2: Create vitest.config.ts**

```typescript
// ui/vitest.config.ts
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
  },
});
```

- [ ] **Step 3: Create test setup file**

```typescript
// ui/src/test-setup.ts
import '@testing-library/jest-dom/vitest';
```

- [ ] **Step 4: Add test script to package.json**

Add to `"scripts"`: `"test": "vitest run"`, `"test:watch": "vitest"`

- [ ] **Step 5: Verify vitest runs (no tests yet)**

```bash
cd /home/dev/git/otelcontext/ui && npx vitest run
```

Expected: "No test files found" or similar (no failures).

- [ ] **Step 6: Commit**

```bash
git add ui/package.json ui/package-lock.json ui/vitest.config.ts ui/src/test-setup.ts
git commit -m "chore(ui): add vitest test infrastructure"
```

---

## Task 2: Tier Layout Algorithm (Pure Logic + Tests)

**Files:**
- Create: `ui/src/lib/tierLayout.ts`
- Create: `ui/src/lib/__tests__/tierLayout.test.ts`

- [ ] **Step 1: Write failing tests for tier layout**

```typescript
// ui/src/lib/__tests__/tierLayout.test.ts
import { describe, it, expect } from 'vitest';
import { assignTiers, computeLayout } from '../tierLayout';

describe('assignTiers', () => {
  it('assigns root nodes (no inbound) to tier 0', () => {
    const nodes = [{ id: 'gateway' }, { id: 'api' }, { id: 'db' }];
    const edges = [
      { source: 'gateway', target: 'api' },
      { source: 'api', target: 'db' },
    ];
    const tiers = assignTiers(nodes, edges);
    expect(tiers.get('gateway')).toBe(0);
  });

  it('assigns leaf nodes (no outbound) to max tier', () => {
    const nodes = [{ id: 'gateway' }, { id: 'api' }, { id: 'db' }];
    const edges = [
      { source: 'gateway', target: 'api' },
      { source: 'api', target: 'db' },
    ];
    const tiers = assignTiers(nodes, edges);
    expect(tiers.get('db')).toBe(3); // max tier (Data)
  });

  it('assigns middle nodes based on longest path from root', () => {
    const nodes = [{ id: 'gw' }, { id: 'auth' }, { id: 'user' }, { id: 'pg' }];
    const edges = [
      { source: 'gw', target: 'auth' },
      { source: 'gw', target: 'user' },
      { source: 'auth', target: 'pg' },
      { source: 'user', target: 'pg' },
    ];
    const tiers = assignTiers(nodes, edges);
    expect(tiers.get('gw')).toBe(0);
    expect(tiers.get('auth')).toBeLessThan(3);
    expect(tiers.get('user')).toBeLessThan(3);
    expect(tiers.get('pg')).toBe(3); // leaf → Data tier
  });

  it('handles cycles by falling back to span_count distribution', () => {
    const nodes = [
      { id: 'a', span_count: 100 },
      { id: 'b', span_count: 50 },
      { id: 'c', span_count: 10 },
      { id: 'd', span_count: 5 },
    ];
    const edges = [
      { source: 'a', target: 'b' },
      { source: 'b', target: 'c' },
      { source: 'c', target: 'a' },
      { source: 'c', target: 'd' },
    ];
    const tiers = assignTiers(nodes, edges);
    // All nodes should have a tier assigned (0-3)
    for (const n of nodes) {
      const t = tiers.get(n.id);
      expect(t).toBeGreaterThanOrEqual(0);
      expect(t).toBeLessThanOrEqual(3);
    }
  });

  it('handles empty graph', () => {
    const tiers = assignTiers([], []);
    expect(tiers.size).toBe(0);
  });

  it('handles single node with no edges', () => {
    const tiers = assignTiers([{ id: 'solo' }], []);
    expect(tiers.get('solo')).toBe(0);
  });
});

describe('computeLayout', () => {
  it('returns x,y coordinates for each node', () => {
    const nodes = [{ id: 'a' }, { id: 'b' }, { id: 'c' }];
    const edges = [
      { source: 'a', target: 'b' },
      { source: 'b', target: 'c' },
    ];
    const layout = computeLayout(nodes, edges, { width: 800, height: 600 });
    expect(layout.get('a')).toEqual(expect.objectContaining({ x: expect.any(Number), y: expect.any(Number) }));
    expect(layout.get('b')).toEqual(expect.objectContaining({ x: expect.any(Number), y: expect.any(Number) }));
    expect(layout.get('c')).toEqual(expect.objectContaining({ x: expect.any(Number), y: expect.any(Number) }));
  });

  it('places higher tiers above lower tiers (smaller y)', () => {
    const nodes = [{ id: 'gw' }, { id: 'svc' }, { id: 'db' }];
    const edges = [
      { source: 'gw', target: 'svc' },
      { source: 'svc', target: 'db' },
    ];
    const layout = computeLayout(nodes, edges, { width: 800, height: 600 });
    const gwY = layout.get('gw')!.y;
    const dbY = layout.get('db')!.y;
    expect(gwY).toBeLessThan(dbY);
  });

  it('spreads nodes horizontally within same tier', () => {
    const nodes = [{ id: 'gw' }, { id: 'a' }, { id: 'b' }, { id: 'c' }];
    const edges = [
      { source: 'gw', target: 'a' },
      { source: 'gw', target: 'b' },
      { source: 'gw', target: 'c' },
    ];
    const layout = computeLayout(nodes, edges, { width: 800, height: 600 });
    const xs = ['a', 'b', 'c'].map((id) => layout.get(id)!.x);
    // All 3 should have distinct x positions
    expect(new Set(xs).size).toBe(3);
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/dev/git/otelcontext/ui && npx vitest run src/lib/__tests__/tierLayout.test.ts
```

Expected: FAIL (module not found)

- [ ] **Step 3: Implement tierLayout.ts**

```typescript
// ui/src/lib/tierLayout.ts

interface NodeInput {
  id: string;
  span_count?: number;
}

interface EdgeInput {
  source: string;
  target: string;
}

const TIER_COUNT = 4; // Gateway=0, API=1, Services=2, Data=3

/**
 * Assign each node to a tier (0-3) using BFS from root nodes.
 * Root nodes = no inbound edges → tier 0 (Gateway).
 * Leaf nodes = no outbound edges → tier 3 (Data).
 * Middle nodes = longest path from any root, bucketed into tiers 1-2.
 * Cycle fallback: sort by span_count descending, distribute evenly.
 */
export function assignTiers(
  nodes: NodeInput[],
  edges: EdgeInput[]
): Map<string, number> {
  if (nodes.length === 0) return new Map();

  const inbound = new Map<string, string[]>();
  const outbound = new Map<string, string[]>();

  for (const n of nodes) {
    inbound.set(n.id, []);
    outbound.set(n.id, []);
  }

  for (const e of edges) {
    outbound.get(e.source)?.push(e.target);
    inbound.get(e.target)?.push(e.source);
  }

  // Find roots (no inbound edges)
  const roots = nodes.filter((n) => inbound.get(n.id)!.length === 0);

  // If no roots (all cycles), fall back to span_count distribution
  if (roots.length === 0) {
    return fallbackTiers(nodes);
  }

  // BFS: longest path from any root
  const depth = new Map<string, number>();
  const queue: string[] = [];

  for (const r of roots) {
    depth.set(r.id, 0);
    queue.push(r.id);
  }

  while (queue.length > 0) {
    const nodeId = queue.shift()!;
    const d = depth.get(nodeId)!;
    for (const target of outbound.get(nodeId) ?? []) {
      const existing = depth.get(target) ?? -1;
      if (d + 1 > existing) {
        depth.set(target, d + 1);
        queue.push(target);
      }
    }
  }

  // Assign any unreached nodes (disconnected) to tier 0
  for (const n of nodes) {
    if (!depth.has(n.id)) depth.set(n.id, 0);
  }

  const maxDepth = Math.max(...depth.values(), 0);

  // Bucket into 4 tiers
  const tiers = new Map<string, number>();
  for (const n of nodes) {
    const d = depth.get(n.id)!;
    const isLeaf = outbound.get(n.id)!.length === 0 && inbound.get(n.id)!.length > 0;

    if (d === 0) {
      tiers.set(n.id, 0); // Gateway
    } else if (isLeaf) {
      tiers.set(n.id, 3); // Data
    } else if (maxDepth <= 2) {
      // Small graph: spread across middle tiers
      tiers.set(n.id, Math.min(d, 2));
    } else {
      // Map depth to tiers 1-2
      const ratio = (d - 1) / Math.max(maxDepth - 1, 1);
      tiers.set(n.id, ratio < 0.5 ? 1 : 2);
    }
  }

  return tiers;
}

function fallbackTiers(nodes: NodeInput[]): Map<string, number> {
  const sorted = [...nodes].sort(
    (a, b) => (b.span_count ?? 0) - (a.span_count ?? 0)
  );
  const tiers = new Map<string, number>();
  const perTier = Math.max(1, Math.ceil(sorted.length / TIER_COUNT));
  sorted.forEach((n, i) => {
    tiers.set(n.id, Math.min(Math.floor(i / perTier), TIER_COUNT - 1));
  });
  return tiers;
}

interface LayoutOptions {
  width: number;
  height: number;
}

interface Position {
  x: number;
  y: number;
  tier: number;
}

/**
 * Compute (x, y) positions for each node based on tier assignments.
 * Tiers are spaced vertically. Nodes within a tier are spaced horizontally.
 */
export function computeLayout(
  nodes: NodeInput[],
  edges: EdgeInput[],
  options: LayoutOptions
): Map<string, Position> {
  const tiers = assignTiers(nodes, edges);
  const { width, height } = options;

  // Group by tier
  const tierGroups = new Map<number, string[]>();
  for (const [id, tier] of tiers) {
    if (!tierGroups.has(tier)) tierGroups.set(tier, []);
    tierGroups.get(tier)!.push(id);
  }

  const positions = new Map<string, Position>();
  const padding = 60;
  const usableHeight = height - padding * 2;
  const usableWidth = width - padding * 2;

  const tierKeys = [...tierGroups.keys()].sort((a, b) => a - b);
  const tierSpacing = tierKeys.length > 1 ? usableHeight / (tierKeys.length - 1) : 0;

  for (let ti = 0; ti < tierKeys.length; ti++) {
    const tier = tierKeys[ti];
    const group = tierGroups.get(tier)!;
    const y = padding + ti * tierSpacing;
    const nodeSpacing =
      group.length > 1 ? usableWidth / (group.length - 1) : 0;

    for (let ni = 0; ni < group.length; ni++) {
      const x =
        group.length > 1
          ? padding + ni * nodeSpacing
          : width / 2; // center single nodes
      positions.set(group[ni], { x, y, tier });
    }
  }

  return positions;
}

/**
 * Compute edge threshold: max(median call_count, 10)
 */
export function defaultEdgeThreshold(
  edges: { call_count: number }[]
): number {
  if (edges.length === 0) return 10;
  const sorted = [...edges].map((e) => e.call_count).sort((a, b) => a - b);
  const median = sorted[Math.floor(sorted.length / 2)];
  return Math.max(median, 10);
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/dev/git/otelcontext/ui && npx vitest run src/lib/__tests__/tierLayout.test.ts
```

Expected: All 8 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/src/lib/tierLayout.ts ui/src/lib/__tests__/tierLayout.test.ts
git commit -m "feat(ui): add tier-based layout algorithm with tests"
```

---

## Task 3: Update Types and Hooks

**Files:**
- Modify: `ui/src/types/api.ts`
- Modify: `ui/src/hooks/useSystemGraph.ts`
- Modify: `ui/src/hooks/useDashboard.ts`
- Create: `ui/src/hooks/__tests__/useSystemGraph.test.ts`

- [ ] **Step 1: Update api.ts types**

In `ui/src/types/api.ts`:
- Add `db_size_mb?: number;` to `RepoStats` interface
- Remove `ServiceMapNode`, `ServiceMapEdge`, `ServiceMapMetrics` interfaces (no longer used)
- Remove `MetricBucket`, `TrafficPoint`, `LatencyPoint` interfaces (metrics page deleted)

- [ ] **Step 2: Add polling to useSystemGraph**

Replace `ui/src/hooks/useSystemGraph.ts` with:

```typescript
import { useCallback, useEffect, useRef, useState } from 'react';
import type { SystemGraphResponse } from '../types/api';

export function useSystemGraph(pollInterval = 60_000) {
  const [graph, setGraph] = useState<SystemGraphResponse | null>(null);
  const [cache, setCache] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const timerRef = useRef<ReturnType<typeof setInterval>>();

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/system/graph');
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      setCache(res.headers.get('X-Cache') ?? '');
      setGraph(await res.json());
      setError(null);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'fetch failed');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    timerRef.current = setInterval(load, pollInterval);
    return () => clearInterval(timerRef.current);
  }, [load, pollInterval]);

  return { graph, cache, loading, error, reload: load };
}
```

- [ ] **Step 3: Add polling to useDashboard**

Replace `ui/src/hooks/useDashboard.ts` with:

```typescript
import { useCallback, useEffect, useRef, useState } from 'react';
import type { DashboardStats, RepoStats } from '../types/api';

export function useDashboard(pollInterval = 30_000) {
  const [dashboard, setDashboard] = useState<DashboardStats | null>(null);
  const [stats, setStats] = useState<RepoStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const timerRef = useRef<ReturnType<typeof setInterval>>();

  const load = useCallback(async () => {
    try {
      const [dRes, sRes] = await Promise.all([
        fetch('/api/metrics/dashboard'),
        fetch('/api/stats'),
      ]);
      if (!dRes.ok || !sRes.ok) throw new Error('fetch failed');
      setDashboard(await dRes.json());
      setStats(await sRes.json());
      setError(null);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'fetch failed');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    timerRef.current = setInterval(load, pollInterval);
    return () => clearInterval(timerRef.current);
  }, [load, pollInterval]);

  return { dashboard, stats, loading, error, reload: load };
}
```

- [ ] **Step 4: Write hook test**

```typescript
// ui/src/hooks/__tests__/useSystemGraph.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

describe('useSystemGraph polling', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('should be importable', async () => {
    const mod = await import('../useSystemGraph');
    expect(mod.useSystemGraph).toBeTypeOf('function');
  });
});
```

- [ ] **Step 5: Run tests**

```bash
cd /home/dev/git/otelcontext/ui && npx vitest run
```

Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add ui/src/types/api.ts ui/src/hooks/useSystemGraph.ts ui/src/hooks/useDashboard.ts ui/src/hooks/__tests__/useSystemGraph.test.ts
git commit -m "feat(ui): update types, add polling to hooks"
```

---

## Task 4: Delete Removed Pages and Hooks

**Files:**
- Delete: `ui/src/components/observability/OverviewPage.tsx`
- Delete: `ui/src/components/observability/MetricsPage.tsx`
- Delete: `ui/src/components/observability/ArchivePage.tsx`
- Delete: `ui/src/components/observability/ServicesPage.tsx`
- Delete: `ui/src/components/shared/JsonViewer.tsx`
- Delete: `ui/src/hooks/useArchive.ts`
- Delete: `ui/src/hooks/useMetrics.ts`

- [ ] **Step 1: Delete files**

```bash
cd /home/dev/git/otelcontext/ui
rm src/components/observability/OverviewPage.tsx
rm src/components/observability/MetricsPage.tsx
rm src/components/observability/ArchivePage.tsx
rm src/components/observability/ServicesPage.tsx
rm src/components/shared/JsonViewer.tsx
rm src/hooks/useArchive.ts
rm src/hooks/useMetrics.ts
```

- [ ] **Step 2: Commit deletions**

```bash
git add -u
git commit -m "chore(ui): remove Overview, Metrics, Archive pages and unused hooks"
```

---

## Task 5: Rewrite TopNav with Global Stats Bar

**Files:**
- Modify: `ui/src/components/nav/TopNav.tsx`
- Modify: `ui/src/styles/global.css`

- [ ] **Step 1: Rewrite TopNav**

Replace `ui/src/components/nav/TopNav.tsx` with a component that:
- Exports `OtelView = 'services' | 'traces' | 'logs' | 'mcp'`
- Has 4 nav items: Service Map (Network icon), Traces (Search icon), Logs (Radar icon), MCP (Terminal icon)
- Right side: stats bar showing `total_traces`, `total_logs`, `active_services`, `error_rate`, `db_size_mb` from dashboard/stats props
- Props: `view`, `onNavigate`, `dashboard`, `stats`, `wsConnected`
- Connection indicator dot (green when wsConnected)

- [ ] **Step 2: Add CSS for stats bar**

Add to `global.css`:

```css
.stats-bar {
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 10px;
  font-family: var(--font-mono, monospace);
  color: var(--text-muted);
}

.stats-bar b { color: var(--text-primary); }
.stats-bar .stat-error { color: #ef4444; }
.stats-bar .stat-healthy { color: #22c55e; }

.ws-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  display: inline-block;
}
.ws-dot.connected { background: #22c55e; }
.ws-dot.disconnected { background: #ef4444; }
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /home/dev/git/otelcontext/ui && npx tsc --noEmit
```

Note: This will fail until App.tsx is updated (Task 6). That's OK — just verify TopNav itself has no type errors.

- [ ] **Step 4: Commit**

```bash
git add ui/src/components/nav/TopNav.tsx ui/src/styles/global.css
git commit -m "feat(ui): rewrite TopNav with 4 views and global stats bar"
```

---

## Task 6: Rewrite App.tsx with Cross-View Navigation

**Files:**
- Modify: `ui/src/App.tsx`

- [ ] **Step 1: Rewrite App.tsx**

Replace `ui/src/App.tsx`. The new version should:
- Import only 4 page components: ServiceMap, TracesPage, LogsPage, MCPConsole
- Use `OtelView` type from TopNav (4 values)
- Add `serviceFilter` state: `useState<string | null>(null)`
- Add navigation callbacks: `navigateToTraces(service)`, `navigateToLogs(service)`, `clearFilter()`
- Pass `serviceFilter` and `onClearFilter` to TracesPage and LogsPage
- Pass `onNavigateToTraces` and `onNavigateToLogs` to ServiceMap
- Pass `dashboard`, `stats`, `wsConnected` to TopNav
- Default view: `'services'`

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/dev/git/otelcontext/ui && npx tsc --noEmit
```

Note: ServiceMap.tsx doesn't exist yet — this will still fail. Proceed to Task 7.

- [ ] **Step 3: Commit**

```bash
git add ui/src/App.tsx
git commit -m "feat(ui): rewrite App with 4 views, cross-view navigation, service filter"
```

---

## Task 7: Service Side Panel Component

**Files:**
- Create: `ui/src/components/observability/ServiceSidePanel.tsx`
- Create: `ui/src/components/observability/__tests__/ServiceSidePanel.test.tsx`

- [ ] **Step 1: Write failing test for side panel**

```typescript
// ui/src/components/observability/__tests__/ServiceSidePanel.test.tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ServiceSidePanel } from '../ServiceSidePanel';
import type { SystemNode, SystemEdge } from '../../../types/api';

const mockNode: SystemNode = {
  id: 'inventory',
  type: 'service',
  health_score: 0.32,
  status: 'critical',
  metrics: {
    request_rate_rps: 670,
    error_rate: 0.084,
    avg_latency_ms: 142,
    p99_latency_ms: 890,
    span_count_1h: 5400,
  },
  alerts: ['error rate above 5%', 'avg latency above 500ms'],
};

const mockEdges: SystemEdge[] = [
  { source: 'user-api', target: 'inventory', call_count: 890, avg_latency_ms: 95, error_rate: 0.03, status: 'degraded' },
  { source: 'inventory', target: 'postgres', call_count: 1200, avg_latency_ms: 12, error_rate: 0, status: 'healthy' },
];

describe('ServiceSidePanel', () => {
  it('renders service name and status badge', () => {
    render(
      <ServiceSidePanel
        node={mockNode}
        edges={mockEdges}
        onClose={() => {}}
        onSelectService={() => {}}
        onViewTraces={() => {}}
        onViewLogs={() => {}}
      />
    );
    expect(screen.getByText('inventory')).toBeInTheDocument();
    expect(screen.getByText('CRITICAL')).toBeInTheDocument();
  });

  it('renders KPI cards with correct values', () => {
    render(
      <ServiceSidePanel
        node={mockNode}
        edges={mockEdges}
        onClose={() => {}}
        onSelectService={() => {}}
        onViewTraces={() => {}}
        onViewLogs={() => {}}
      />
    );
    expect(screen.getByText('670')).toBeInTheDocument(); // RPS
    expect(screen.getByText('8.4%')).toBeInTheDocument(); // Error rate
    expect(screen.getByText('142ms')).toBeInTheDocument(); // Avg latency
    expect(screen.getByText('890ms')).toBeInTheDocument(); // P99
  });

  it('renders upstream and downstream services', () => {
    render(
      <ServiceSidePanel
        node={mockNode}
        edges={mockEdges}
        onClose={() => {}}
        onSelectService={() => {}}
        onViewTraces={() => {}}
        onViewLogs={() => {}}
      />
    );
    expect(screen.getByText('user-api')).toBeInTheDocument();
    expect(screen.getByText('postgres')).toBeInTheDocument();
  });

  it('renders alerts', () => {
    render(
      <ServiceSidePanel
        node={mockNode}
        edges={mockEdges}
        onClose={() => {}}
        onSelectService={() => {}}
        onViewTraces={() => {}}
        onViewLogs={() => {}}
      />
    );
    expect(screen.getByText('error rate above 5%')).toBeInTheDocument();
  });

  it('calls onViewTraces when action link clicked', () => {
    const onViewTraces = vi.fn();
    render(
      <ServiceSidePanel
        node={mockNode}
        edges={mockEdges}
        onClose={() => {}}
        onSelectService={() => {}}
        onViewTraces={onViewTraces}
        onViewLogs={() => {}}
      />
    );
    fireEvent.click(screen.getByText(/View Traces/));
    expect(onViewTraces).toHaveBeenCalledWith('inventory');
  });

  it('calls onSelectService when upstream service clicked', () => {
    const onSelectService = vi.fn();
    render(
      <ServiceSidePanel
        node={mockNode}
        edges={mockEdges}
        onClose={() => {}}
        onSelectService={onSelectService}
        onViewTraces={() => {}}
        onViewLogs={() => {}}
      />
    );
    fireEvent.click(screen.getByText('user-api'));
    expect(onSelectService).toHaveBeenCalledWith('user-api');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/dev/git/otelcontext/ui && npx vitest run src/components/observability/__tests__/ServiceSidePanel.test.tsx
```

Expected: FAIL (module not found)

- [ ] **Step 3: Implement ServiceSidePanel.tsx**

Create `ui/src/components/observability/ServiceSidePanel.tsx` — a React component with:
- Props: `node: SystemNode`, `edges: SystemEdge[]`, `onClose`, `onSelectService(id)`, `onViewTraces(service)`, `onViewLogs(service)`
- Header: health dot + service name + status badge + close X button
- 2x2 KPI grid: RPS, Error Rate (formatted as %), Avg Latency (ms), P99 (ms)
- Health score progress bar (0-1.0, color coded)
- Upstream list (edges where `target === node.id`), clickable
- Downstream list (edges where `source === node.id`), clickable
- Alerts list from `node.alerts`
- Action links: "View Traces →", "View Logs →"
- Uses inline styles matching the design spec color system
- Wrapped in `React.memo`

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/dev/git/otelcontext/ui && npx vitest run src/components/observability/__tests__/ServiceSidePanel.test.tsx
```

Expected: All 6 tests PASS.

- [ ] **Step 5: Add side panel CSS to global.css**

```css
.side-panel {
  width: 280px;
  border-left: 1px solid var(--border);
  background: var(--bg-card);
  overflow-y: auto;
  padding: 12px;
  flex-shrink: 0;
}

.side-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.kpi-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
  margin-bottom: 12px;
}

.kpi-card {
  background: var(--bg-panel);
  border-radius: 6px;
  padding: 8px;
}

.kpi-label {
  font-size: 8px;
  color: var(--text-muted);
  text-transform: uppercase;
  margin-bottom: 2px;
}

.kpi-value {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-primary);
}

.health-bar {
  height: 4px;
  background: var(--border);
  border-radius: 2px;
  overflow: hidden;
}

.connection-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 8px;
  background: var(--bg-panel);
  border-radius: 4px;
  font-size: 10px;
  cursor: pointer;
}

.connection-item:hover {
  background: var(--bg-input);
}

.alert-item {
  padding: 4px 8px;
  background: #1c0707;
  border-radius: 4px;
  font-size: 9px;
  color: #fca5a5;
}

.action-link {
  font-size: 9px;
  padding: 4px 8px;
  background: #1e3a5f;
  color: #38bdf8;
  border-radius: 4px;
  cursor: pointer;
  border: none;
}

.action-link:hover {
  background: #1e4a7f;
}
```

- [ ] **Step 6: Commit**

```bash
git add ui/src/components/observability/ServiceSidePanel.tsx ui/src/components/observability/__tests__/ServiceSidePanel.test.tsx ui/src/styles/global.css
git commit -m "feat(ui): add ServiceSidePanel component with tests"
```

---

## Task 8: Service Map Component (Main Rewrite)

**Files:**
- Create: `ui/src/components/observability/ServiceMap.tsx`
- Modify: `ui/src/styles/global.css`

- [ ] **Step 1: Create ServiceMap.tsx**

Create `ui/src/components/observability/ServiceMap.tsx` — the hero component. Structure:

**Props:**
```typescript
interface ServiceMapProps {
  graph: SystemGraphResponse | null;
  cache: string;
  loading: boolean;
  error: string | null;
  onNavigateToTraces: (service: string) => void;
  onNavigateToLogs: (service: string) => void;
}
```

**Internal state:**
- `selectedNode: SystemNode | null`
- `edgeThreshold: number` (slider value)
- `searchQuery: string` (debounced)
- `zoomLevel: number` (for semantic zoom)

**Layout:**
- Toolbar row: search input + edge threshold slider + zoom buttons (+/−/Fit)
- Main area: flex row with ECharts canvas (flex: 1) + side panel (280px, conditional)
- Minimap: absolute-positioned bottom-right in canvas area

**ECharts config:**
- `series[0].type: 'graph'`
- `series[0].layout: 'none'` (fixed positions from `computeLayout`)
- `series[0].data`: nodes with `x`, `y`, `symbol: 'roundRect'`, `symbolSize`, custom `itemStyle` per health status
- `series[0].links`: filtered edges above threshold, max 500
- `series[0].label`: show name + metrics (hidden at low zoom)
- `series[0].edgeSymbol: ['none', 'arrow']`
- `series[0].lineStyle`: width from `log(call_count)`, color by health
- `roam: true` for pan/zoom
- `tooltip` with service details on hover
- Listen to `'click'` event → set selectedNode
- Listen to `'datazoom'` / `'georoam'` for zoom level → semantic zoom

**Edge threshold slider:**
- Range input, min=1, max=maxCallCount
- Default: `defaultEdgeThreshold(edges)`
- On change: refilter edges passed to ECharts

**Search:**
- Debounced 300ms text input
- Matching nodes: `emphasis` state in ECharts
- Non-matching: reduced opacity via `itemStyle.opacity`

**Minimap:**
- Second `EChart` component (existing shared component)
- Same node positions, dots only (symbolSize: 4), no labels, no edges
- Viewport rectangle as ECharts `graphic` element
- `silent: true` except for click handler

**Tier labels:**
- ECharts `graphic` text elements positioned at left edge, one per tier

- [ ] **Step 2: Add service map CSS to global.css**

```css
.service-map-container {
  display: flex;
  flex-direction: column;
  flex: 1;
  overflow: hidden;
}

.service-map-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border);
}

.service-map-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.service-map-canvas {
  flex: 1;
  position: relative;
  min-height: 0;
}

.service-map-minimap {
  position: absolute;
  bottom: 8px;
  right: 8px;
  width: 100px;
  height: 80px;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 4px;
  opacity: 0.8;
  z-index: 10;
}

.edge-slider {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 10px;
  color: var(--text-muted);
}

.edge-slider input[type="range"] {
  width: 120px;
  accent-color: var(--color-accent);
}

.zoom-controls {
  display: flex;
  gap: 4px;
}

.zoom-btn {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 12px;
}

.zoom-btn:hover {
  border-color: var(--border-hover);
  color: var(--text-primary);
}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /home/dev/git/otelcontext/ui && npx tsc --noEmit
```

Expected: PASS (all imports should resolve now).

- [ ] **Step 4: Run all tests**

```bash
cd /home/dev/git/otelcontext/ui && npx vitest run
```

Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add ui/src/components/observability/ServiceMap.tsx ui/src/styles/global.css
git commit -m "feat(ui): add hierarchical ServiceMap with ECharts, side panel, search, minimap"
```

---

## Task 9: Update TracesPage and LogsPage with Service Filter

**Files:**
- Modify: `ui/src/components/observability/TracesPage.tsx`
- Modify: `ui/src/components/observability/LogsPage.tsx`

- [ ] **Step 1: Update TracesPage**

In `ui/src/components/observability/TracesPage.tsx`:
- Add props: `serviceFilter: string | null`, `onClearFilter: () => void`
- Remove `JsonViewer` import and usage
- Add filter chip above trace list when `serviceFilter` is set: shows `"Filtered: {serviceName}"` with an X button calling `onClearFilter`
- Filter `traces` array by `serviceFilter` (case-insensitive match on `service_name`)

- [ ] **Step 2: Update LogsPage**

In `ui/src/components/observability/LogsPage.tsx`:
- Add props: `serviceFilter: string | null`, `onClearFilter: () => void`
- Add filter chip above search input when `serviceFilter` is set
- Filter `logs` array by `serviceFilter`

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /home/dev/git/otelcontext/ui && npx tsc --noEmit
```

Expected: PASS.

- [ ] **Step 4: Run all tests**

```bash
cd /home/dev/git/otelcontext/ui && npx vitest run
```

Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add ui/src/components/observability/TracesPage.tsx ui/src/components/observability/LogsPage.tsx
git commit -m "feat(ui): add service filter to TracesPage and LogsPage"
```

---

## Task 10: Build, Embed UI, and Verify

**Files:**
- Modify: `ui/src/styles/global.css` (any remaining cleanup)

- [ ] **Step 1: Run all tests**

```bash
cd /home/dev/git/otelcontext/ui && npx vitest run
```

Expected: All tests pass.

- [ ] **Step 2: Build UI**

```bash
cd /home/dev/git/otelcontext/ui && npm run build
```

Expected: Build succeeds with no errors.

- [ ] **Step 3: Copy built assets to embedded UI directory**

```bash
cp -r /home/dev/git/otelcontext/ui/dist/* /home/dev/git/otelcontext/internal/ui/dist/
```

- [ ] **Step 4: Build Go binary**

```bash
cd /home/dev/git/otelcontext && go build -o otelcontext .
```

Expected: Build succeeds.

- [ ] **Step 5: Commit all changes**

```bash
cd /home/dev/git/otelcontext
git add ui/ internal/ui/dist/
git commit -m "feat(ui): build and embed redesigned UI"
```

---

## Task 11: Tag Beta and Push to GitHub

- [ ] **Step 1: Run final test suite**

```bash
cd /home/dev/git/otelcontext/ui && npx vitest run
cd /home/dev/git/otelcontext && go vet ./...
```

Expected: All pass.

- [ ] **Step 2: Create beta tag**

```bash
cd /home/dev/git/otelcontext
git tag -a v0.2.0-beta.1 -m "feat: UI redesign - hierarchical service map, 4-view navigation, progressive edge disclosure"
```

- [ ] **Step 3: Push to GitHub**

```bash
git push origin main --tags
```

- [ ] **Step 4: Verify push**

```bash
gh release list --limit 5 2>/dev/null || echo "No releases"
git log --oneline -5
```
