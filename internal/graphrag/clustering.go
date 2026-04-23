package graphrag

// Log clustering is now performed by the Drain template miner (see drain.go).
// processLog() in builder.go calls GraphRAG.clusterLog() which delegates to
// the shared *Drain instance. The vectordb.Index (TF-IDF) is still used for
// SimilarErrors — similarity search across mined templates.

import (
	"fmt"
	"time"
)

// clusterLog runs the log body through Drain and upserts a LogClusterNode
// into the SignalStore. Returns the service-scoped cluster ID.
//
// The cluster ID is service-scoped and derived from the Drain template ID,
// so it remains stable for any future ingestion of the same template shape.
// Drain's internal template ID may change when tokens generalize to
// wildcards; the LogClusterNode's TemplateID field is updated to track this.
func (g *GraphRAG) clusterLog(service, body, severity string, ts time.Time) string {
	if g.drain == nil {
		// Fallback: legacy hash-based clustering.
		clusterID := fmt.Sprintf("lc_%s_%x", service, simpleHash(body))
		g.SignalStore.UpsertLogCluster(clusterID, body, severity, service, ts)
		return clusterID
	}

	tpl := g.drain.Match(body, ts)
	if tpl == nil {
		return ""
	}
	// Service-scoped stable cluster ID derived from the first-seen template
	// tokens. We use the current template ID as a deterministic suffix; when
	// Drain merges (tokens generalize), the ID may shift — acceptable since
	// it occurs only on the first few ingestions of a pattern.
	clusterID := fmt.Sprintf("lc_%s_%x", service, tpl.ID)
	g.SignalStore.UpsertLogClusterWithTemplate(
		clusterID,
		tpl.TemplateString(),
		severity,
		service,
		tpl.ID,
		tpl.Tokens,
		tpl.Sample,
		ts,
	)
	return clusterID
}

// SimilarErrors finds log clusters similar to a given cluster using the vector index.
func (g *GraphRAG) SimilarErrors(clusterID string, k int) []LogClusterNode {
	if k <= 0 {
		k = 10
	}

	g.SignalStore.mu.RLock()
	cluster, ok := g.SignalStore.LogClusters[clusterID]
	g.SignalStore.mu.RUnlock()
	if !ok {
		return nil
	}

	// Use vectordb to find similar logs based on the mined template.
	if g.vectorIdx == nil {
		return nil
	}
	query := cluster.Template
	if query == "" && len(cluster.TemplateTokens) > 0 {
		query = joinTokens(cluster.TemplateTokens)
	}
	results := g.vectorIdx.Search(query, k*2) // over-fetch to filter

	// Map results back to log clusters.
	seen := map[string]bool{clusterID: true}
	var similar []LogClusterNode

	g.SignalStore.mu.RLock()
	defer g.SignalStore.mu.RUnlock()

	for _, r := range results {
		for _, lc := range g.SignalStore.LogClusters {
			if seen[lc.ID] {
				continue
			}
			for _, e := range g.SignalStore.Edges {
				if e.Type == EdgeEmittedBy && e.FromID == lc.ID && e.ToID == r.ServiceName {
					seen[lc.ID] = true
					similar = append(similar, *lc)
					break
				}
			}
			if len(similar) >= k {
				break
			}
		}
		if len(similar) >= k {
			break
		}
	}

	return similar
}

// joinTokens is a tiny helper to avoid importing strings in this file's
// hot path; equivalent to strings.Join(tokens, " ").
func joinTokens(tokens []string) string {
	n := 0
	for _, t := range tokens {
		n += len(t) + 1
	}
	b := make([]byte, 0, n)
	for i, t := range tokens {
		if i > 0 {
			b = append(b, ' ')
		}
		b = append(b, t...)
	}
	return string(b)
}
