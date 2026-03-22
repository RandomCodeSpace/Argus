export interface NodeInput {
  id: string;
  span_count?: number;
}

export interface EdgeInput {
  source: string;
  target: string;
}

export interface Position {
  x: number;
  y: number;
  tier: number;
}

/**
 * Assign each node to a tier (0 = Gateway/root, 3 = Data/leaf).
 *
 * Algorithm:
 * 1. Build inbound/outbound adjacency maps
 * 2. Find roots (no inbound). If none (cycles), fallback by span_count
 * 3. BFS from roots, assign longest-path depth
 * 4. Leaf nodes (outbound=0, inbound>0) forced to tier 3
 * 5. Middle nodes bucketed into tiers 1-2 by depth ratio
 */
export function assignTiers(
  nodes: NodeInput[],
  edges: EdgeInput[],
): Map<string, number> {
  const result = new Map<string, number>();

  if (nodes.length === 0) return result;
  if (nodes.length === 1) {
    result.set(nodes[0].id, 0);
    return result;
  }

  const nodeIds = new Set(nodes.map((n) => n.id));
  const inbound = new Map<string, string[]>();
  const outbound = new Map<string, string[]>();

  for (const id of nodeIds) {
    inbound.set(id, []);
    outbound.set(id, []);
  }

  for (const edge of edges) {
    if (!nodeIds.has(edge.source) || !nodeIds.has(edge.target)) continue;
    outbound.get(edge.source)!.push(edge.target);
    inbound.get(edge.target)!.push(edge.source);
  }

  // Find roots: nodes with no inbound edges
  const roots = nodes.filter((n) => inbound.get(n.id)!.length === 0);

  if (roots.length === 0) {
    // Cycle fallback: sort by span_count desc, distribute across 4 tiers
    const sorted = [...nodes].sort(
      (a, b) => (b.span_count ?? 0) - (a.span_count ?? 0),
    );
    const tierSize = Math.max(1, Math.ceil(sorted.length / 4));
    for (let i = 0; i < sorted.length; i++) {
      const tier = Math.min(3, Math.floor(i / tierSize));
      result.set(sorted[i].id, tier);
    }
    return result;
  }

  // BFS from roots — longest path depth
  const depth = new Map<string, number>();
  for (const id of nodeIds) depth.set(id, 0);

  const queue: string[] = roots.map((r) => r.id);
  // Use longest-path BFS: revisit if we find a longer path
  for (const r of roots) depth.set(r.id, 0);

  let head = 0;
  while (head < queue.length) {
    const current = queue[head++];
    const currentDepth = depth.get(current)!;
    for (const neighbor of outbound.get(current) ?? []) {
      const newDepth = currentDepth + 1;
      if (newDepth > depth.get(neighbor)!) {
        depth.set(neighbor, newDepth);
        queue.push(neighbor);
      }
    }
  }

  const maxDepth = Math.max(...[...depth.values()]);

  for (const node of nodes) {
    const id = node.id;
    const inboundEdges = inbound.get(id)!;
    const outboundEdges = outbound.get(id)!;
    const d = depth.get(id)!;

    if (inboundEdges.length === 0) {
      // Root node
      result.set(id, 0);
    } else if (outboundEdges.length === 0 && inboundEdges.length > 0) {
      // Leaf node
      result.set(id, 3);
    } else if (maxDepth <= 1) {
      // Very shallow graph, middle nodes go to tier 1
      result.set(id, 1);
    } else {
      // Middle node: bucket into tiers 1-2 based on depth ratio
      const ratio = d / maxDepth;
      // ratio 0 would be root (handled above), ratio 1 would be leaf (handled above)
      // Map (0,1) to tiers 1-2
      const tier = ratio <= 0.5 ? 1 : 2;
      result.set(id, tier);
    }
  }

  return result;
}

const PADDING = 60;

/**
 * Compute x,y positions for each node based on tier assignment.
 * Tier 0 at the top (smallest y), tier 3 at the bottom (largest y).
 * Nodes within the same tier are spaced horizontally.
 */
export function computeLayout(
  nodes: NodeInput[],
  edges: EdgeInput[],
  options: { width: number; height: number },
): Map<string, Position> {
  const result = new Map<string, Position>();
  if (nodes.length === 0) return result;

  const tiers = assignTiers(nodes, edges);
  const maxTier = Math.max(...[...tiers.values()]);

  // Group nodes by tier
  const groups = new Map<number, string[]>();
  for (const [id, tier] of tiers) {
    if (!groups.has(tier)) groups.set(tier, []);
    groups.get(tier)!.push(id);
  }

  const tierCount = maxTier + 1;
  const availableHeight = options.height - 2 * PADDING;
  const tierSpacing =
    tierCount > 1 ? availableHeight / (tierCount - 1) : 0;

  for (const [tier, ids] of groups) {
    const y =
      tierCount > 1 ? PADDING + tier * tierSpacing : options.height / 2;

    const availableWidth = options.width - 2 * PADDING;
    const nodeSpacing =
      ids.length > 1 ? availableWidth / (ids.length - 1) : 0;

    for (let i = 0; i < ids.length; i++) {
      const x =
        ids.length > 1 ? PADDING + i * nodeSpacing : options.width / 2;
      result.set(ids[i], { x, y, tier });
    }
  }

  return result;
}

/**
 * Compute an edge visibility threshold: max(median call_count, 10).
 */
export function defaultEdgeThreshold(
  edges: { call_count: number }[],
): number {
  if (edges.length === 0) return 10;

  const sorted = [...edges].map((e) => e.call_count).sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  const median =
    sorted.length % 2 === 0
      ? (sorted[mid - 1] + sorted[mid]) / 2
      : sorted[mid];

  return Math.max(median, 10);
}
