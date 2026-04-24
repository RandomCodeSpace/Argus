package graphrag

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/RandomCodeSpace/otelcontext/internal/storage"
)

// investigationCooldown suppresses repeated PersistInvestigation calls with
// the same (trigger_service, root_service, root_operation) inside a sliding
// window. Without this, a single stuck service produces one insert every
// anomaly tick (default 10s) indefinitely.
type investigationCooldown struct {
	mu       sync.Mutex
	lastSeen map[string]time.Time
	window   time.Duration
}

func newInvestigationCooldown(window time.Duration) *investigationCooldown {
	return &investigationCooldown{
		lastSeen: map[string]time.Time{},
		window:   window,
	}
}

// allow returns true when the key has not been seen within the sliding
// window. On allow, it records now as the new last-seen timestamp.
func (c *investigationCooldown) allow(key string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t, ok := c.lastSeen[key]; ok && now.Sub(t) < c.window {
		return false
	}
	c.lastSeen[key] = now
	return true
}

// cooldownKey builds a case- and whitespace-insensitive key from the tuple
// (tenant, trigger_service, root_service, root_operation). Tenant scopes the
// guard so an error in tenant-A doesn't suppress the same error pattern in
// tenant-B. Service names emitted from different instrumentations occasionally
// differ in casing or have trailing whitespace; canonicalizing here prevents
// those variants from bypassing the cooldown guard.
func cooldownKey(tenant, triggerService, rootService, rootOperation string) string {
	norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	return norm(tenant) + "|" + norm(triggerService) + "|" + norm(rootService) + "|" + norm(rootOperation)
}

// prune drops entries older than cutoff to bound map size. Called from
// the refresh tick.
func (c *investigationCooldown) prune(cutoff time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, t := range c.lastSeen {
		if t.Before(cutoff) {
			delete(c.lastSeen, k)
		}
	}
}

// Investigation is a persisted record of an automated error investigation.
//
// TenantID scopes the row to its originating tenant. The composite
// (tenant_id, created_at) index supports the recency-ordered "investigations
// for tenant X" query that GetInvestigations runs on every read.
type Investigation struct {
	TenantID         string          `gorm:"size:64;default:'default';not null;index:idx_investigations_tenant_created,priority:1" json:"tenant_id"`
	ID               string          `gorm:"primaryKey;size:64" json:"id"`
	CreatedAt        time.Time       `gorm:"index:idx_investigations_tenant_created,priority:2" json:"created_at"`
	Status           string          `gorm:"size:20" json:"status"`   // detected, triaged, resolved
	Severity         string          `gorm:"size:20" json:"severity"` // critical, warning, info
	TriggerService   string          `gorm:"size:255;index" json:"trigger_service"`
	TriggerOperation string          `gorm:"size:255" json:"trigger_operation"`
	ErrorMessage     string          `gorm:"type:text" json:"error_message"`
	RootService      string          `gorm:"size:255" json:"root_service"`
	RootOperation    string          `gorm:"size:255" json:"root_operation"`
	CausalChain      json.RawMessage `gorm:"type:text" json:"causal_chain"`
	TraceIDs         json.RawMessage `gorm:"type:text" json:"trace_ids"`
	ErrorLogs        json.RawMessage `gorm:"type:text" json:"error_logs"`
	AnomalousMetrics json.RawMessage `gorm:"type:text" json:"anomalous_metrics"`
	AffectedServices json.RawMessage `gorm:"type:text" json:"affected_services"`
	SpanChain        json.RawMessage `gorm:"type:text" json:"span_chain"`
}

// TableName overrides GORM's default table name.
func (Investigation) TableName() string {
	return "investigations"
}

// PersistInvestigation saves an investigation record from an error chain
// analysis. Tenant is accepted explicitly so the caller (the per-tenant
// anomaly loop) can re-enter ImpactAnalysis on the correct tenant slice and so
// the persisted row carries its originating tenant_id.
func (g *GraphRAG) PersistInvestigation(tenant, triggerService string, chains []ErrorChainResult, anomalies []*AnomalyNode) {
	if len(chains) == 0 {
		return
	}
	if tenant == "" {
		tenant = storage.DefaultTenantID
	}

	firstChain := chains[0]
	if firstChain.RootCause == nil {
		return
	}

	now := time.Now()

	// Cooldown: suppress repeat investigations for the same
	// (tenant, trigger_service, root_service, root_operation) inside a sliding
	// window. Keys are canonicalized (lower + trim) so "Orders" and "orders "
	// share a bucket — otherwise trivial casing differences would bypass the
	// guard. Tenant scoping prevents an error in one tenant from suppressing
	// the same pattern in another.
	key := cooldownKey(tenant, triggerService, firstChain.RootCause.Service, firstChain.RootCause.Operation)
	if g.invCooldown != nil && !g.invCooldown.allow(key, now) {
		return
	}
	// Increment BEFORE db.Create so the counter reflects "cooldown allowed;
	// persist attempted". See InvestigationInsertCount's doc comment.
	g.invInserts.Add(1)

	id := fmt.Sprintf("inv_%d", now.UnixNano())

	severity := "warning"
	if len(anomalies) > 0 {
		for _, a := range anomalies {
			if a.Severity == SeverityCritical {
				severity = "critical"
				break
			}
		}
	}

	// Collect trace IDs
	var traceIDs []string
	for _, c := range chains {
		traceIDs = append(traceIDs, c.TraceID)
	}

	// Build causal chain
	type causalStep struct {
		Service   string `json:"service"`
		Operation string `json:"operation"`
		SpanID    string `json:"span_id"`
		IsError   bool   `json:"is_error"`
	}
	var causal []causalStep
	for _, s := range firstChain.SpanChain {
		causal = append(causal, causalStep{
			Service:   s.Service,
			Operation: s.Operation,
			SpanID:    s.ID,
			IsError:   s.IsError,
		})
	}

	// Affected services from impact analysis, run against the tenant that
	// raised this investigation.
	impactCtx := storage.WithTenantContext(context.Background(), tenant)
	impact := g.ImpactAnalysis(impactCtx, triggerService, 3)
	var affected []string
	for _, a := range impact.AffectedServices {
		affected = append(affected, a.Service)
	}

	causalJSON, _ := json.Marshal(causal)
	traceJSON, _ := json.Marshal(traceIDs)
	logsJSON, _ := json.Marshal(firstChain.CorrelatedLogs)
	affectedJSON, _ := json.Marshal(affected)
	spanJSON, _ := json.Marshal(firstChain.SpanChain)

	var metricsJSON []byte
	if len(firstChain.AnomalousMetrics) > 0 {
		metricsJSON, _ = json.Marshal(firstChain.AnomalousMetrics)
	} else {
		metricsJSON = []byte("[]")
	}

	inv := Investigation{
		TenantID:         tenant,
		ID:               id,
		CreatedAt:        now,
		Status:           "detected",
		Severity:         severity,
		TriggerService:   triggerService,
		TriggerOperation: firstChain.RootCause.Operation,
		ErrorMessage:     firstChain.RootCause.ErrorMessage,
		RootService:      firstChain.RootCause.Service,
		RootOperation:    firstChain.RootCause.Operation,
		CausalChain:      causalJSON,
		TraceIDs:         traceJSON,
		ErrorLogs:        logsJSON,
		AnomalousMetrics: metricsJSON,
		AffectedServices: affectedJSON,
		SpanChain:        spanJSON,
	}

	if g.repo == nil || g.repo.DB() == nil {
		// No repo (e.g., test harness): cooldown still applied; skip DB write.
		return
	}
	if err := g.repo.DB().Create(&inv).Error; err != nil {
		slog.Error("Failed to persist investigation", "tenant", tenant, "error", err)
		return
	}

	slog.Info("Investigation persisted", "id", id, "tenant", tenant, "service", triggerService, "severity", severity)
}

// GetInvestigations queries persisted investigations scoped to the tenant
// carried by ctx. The composite (tenant_id, created_at) index supports the
// recency-ordered scan.
func (g *GraphRAG) GetInvestigations(ctx context.Context, service, severity, status string, limit int) ([]Investigation, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	tenant := storage.TenantFromContext(ctx)
	db := g.repo.DB().
		Model(&Investigation{}).
		Where("tenant_id = ?", tenant).
		Order("created_at DESC").
		Limit(limit)
	if service != "" {
		db = db.Where("trigger_service = ? OR root_service = ?", service, service)
	}
	if severity != "" {
		db = db.Where("severity = ?", severity)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}

	var investigations []Investigation
	if err := db.Find(&investigations).Error; err != nil {
		return nil, err
	}
	return investigations, nil
}

// GetInvestigation retrieves a single investigation by ID, scoped to the
// tenant carried by ctx. Returning ErrRecordNotFound for cross-tenant lookups
// prevents id-guessing from leaking another tenant's row.
func (g *GraphRAG) GetInvestigation(ctx context.Context, id string) (*Investigation, error) {
	tenant := storage.TenantFromContext(ctx)
	var inv Investigation
	if err := g.repo.DB().Where("tenant_id = ? AND id = ?", tenant, id).First(&inv).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}
