import { describe, it, expect } from 'vitest';
import {
  assignTiers,
  computeLayout,
  defaultEdgeThreshold,
  NodeInput,
  EdgeInput,
} from '../tierLayout';

describe('assignTiers', () => {
  it('returns empty map for empty graph', () => {
    const result = assignTiers([], []);
    expect(result.size).toBe(0);
  });

  it('assigns tier 0 to a single node', () => {
    const result = assignTiers([{ id: 'A' }], []);
    expect(result.get('A')).toBe(0);
  });

  it('assigns tier 0 to root nodes (no inbound edges)', () => {
    const nodes: NodeInput[] = [{ id: 'A' }, { id: 'B' }, { id: 'C' }];
    const edges: EdgeInput[] = [
      { source: 'A', target: 'B' },
      { source: 'B', target: 'C' },
    ];
    const result = assignTiers(nodes, edges);
    expect(result.get('A')).toBe(0);
  });

  it('assigns tier 3 to leaf nodes (no outbound, has inbound)', () => {
    const nodes: NodeInput[] = [{ id: 'A' }, { id: 'B' }, { id: 'C' }];
    const edges: EdgeInput[] = [
      { source: 'A', target: 'B' },
      { source: 'B', target: 'C' },
    ];
    const result = assignTiers(nodes, edges);
    expect(result.get('C')).toBe(3);
  });

  it('assigns intermediate tiers to middle nodes', () => {
    const nodes: NodeInput[] = [
      { id: 'gateway' },
      { id: 'service' },
      { id: 'db' },
    ];
    const edges: EdgeInput[] = [
      { source: 'gateway', target: 'service' },
      { source: 'service', target: 'db' },
    ];
    const result = assignTiers(nodes, edges);
    expect(result.get('gateway')).toBe(0);
    const serviceTier = result.get('service')!;
    expect(serviceTier).toBeGreaterThan(0);
    expect(serviceTier).toBeLessThan(3);
    expect(result.get('db')).toBe(3);
  });

  it('uses cycle fallback when all nodes form a cycle', () => {
    const nodes: NodeInput[] = [
      { id: 'A', span_count: 100 },
      { id: 'B', span_count: 50 },
      { id: 'C', span_count: 10 },
      { id: 'D', span_count: 1 },
    ];
    const edges: EdgeInput[] = [
      { source: 'A', target: 'B' },
      { source: 'B', target: 'C' },
      { source: 'C', target: 'D' },
      { source: 'D', target: 'A' },
    ];
    const result = assignTiers(nodes, edges);

    // All nodes have inbound edges, so cycle fallback triggers
    // Sorted by span_count desc: A(100), B(50), C(10), D(1)
    // Distributed across 4 tiers
    expect(result.get('A')).toBe(0);
    expect(result.get('D')).toBeGreaterThan(result.get('A')!);
    // Every node should have a tier
    expect(result.size).toBe(4);
  });

  it('handles deeper graphs with multiple middle tiers', () => {
    const nodes: NodeInput[] = [
      { id: 'root' },
      { id: 'm1' },
      { id: 'm2' },
      { id: 'm3' },
      { id: 'leaf' },
    ];
    const edges: EdgeInput[] = [
      { source: 'root', target: 'm1' },
      { source: 'm1', target: 'm2' },
      { source: 'm2', target: 'm3' },
      { source: 'm3', target: 'leaf' },
    ];
    const result = assignTiers(nodes, edges);
    expect(result.get('root')).toBe(0);
    expect(result.get('leaf')).toBe(3);
    // Middle nodes should be between 0 and 3
    for (const mid of ['m1', 'm2', 'm3']) {
      const t = result.get(mid)!;
      expect(t).toBeGreaterThanOrEqual(1);
      expect(t).toBeLessThanOrEqual(2);
    }
  });
});

describe('computeLayout', () => {
  it('returns empty map for empty graph', () => {
    const result = computeLayout([], [], { width: 800, height: 600 });
    expect(result.size).toBe(0);
  });

  it('returns x, y, tier for each node', () => {
    const nodes: NodeInput[] = [{ id: 'A' }, { id: 'B' }];
    const edges: EdgeInput[] = [{ source: 'A', target: 'B' }];
    const result = computeLayout(nodes, edges, { width: 800, height: 600 });

    expect(result.size).toBe(2);
    for (const pos of result.values()) {
      expect(pos).toHaveProperty('x');
      expect(pos).toHaveProperty('y');
      expect(pos).toHaveProperty('tier');
    }
  });

  it('higher tiers have larger y values (tier 0 at top)', () => {
    const nodes: NodeInput[] = [{ id: 'A' }, { id: 'B' }, { id: 'C' }];
    const edges: EdgeInput[] = [
      { source: 'A', target: 'B' },
      { source: 'B', target: 'C' },
    ];
    const result = computeLayout(nodes, edges, { width: 800, height: 600 });

    const posA = result.get('A')!;
    const posC = result.get('C')!;
    // Tier 0 (A) should be at top (smaller y), tier 3 (C) at bottom (larger y)
    expect(posA.y).toBeLessThan(posC.y);
  });

  it('same-tier nodes have distinct x positions', () => {
    // Two roots, no edges between them
    const nodes: NodeInput[] = [
      { id: 'A' },
      { id: 'B' },
      { id: 'C' },
      { id: 'D' },
    ];
    const edges: EdgeInput[] = [
      { source: 'A', target: 'C' },
      { source: 'B', target: 'D' },
    ];
    const result = computeLayout(nodes, edges, { width: 800, height: 600 });

    // A and B are both roots (tier 0)
    const posA = result.get('A')!;
    const posB = result.get('B')!;
    expect(posA.tier).toBe(posB.tier);
    expect(posA.x).not.toBe(posB.x);
  });

  it('centers a single node', () => {
    const result = computeLayout([{ id: 'X' }], [], {
      width: 800,
      height: 600,
    });
    const pos = result.get('X')!;
    expect(pos.x).toBe(400);
  });
});

describe('defaultEdgeThreshold', () => {
  it('returns 10 for empty edges', () => {
    expect(defaultEdgeThreshold([])).toBe(10);
  });

  it('returns 10 when median is below 10', () => {
    const edges = [{ call_count: 1 }, { call_count: 2 }, { call_count: 3 }];
    expect(defaultEdgeThreshold(edges)).toBe(10);
  });

  it('returns median when median exceeds 10', () => {
    const edges = [
      { call_count: 5 },
      { call_count: 20 },
      { call_count: 100 },
    ];
    // median = 20
    expect(defaultEdgeThreshold(edges)).toBe(20);
  });

  it('averages two middle values for even-length arrays', () => {
    const edges = [
      { call_count: 10 },
      { call_count: 20 },
      { call_count: 30 },
      { call_count: 40 },
    ];
    // median = (20+30)/2 = 25
    expect(defaultEdgeThreshold(edges)).toBe(25);
  });
});
