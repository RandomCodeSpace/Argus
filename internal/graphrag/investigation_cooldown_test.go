package graphrag

import (
	"testing"
)

// TestPersistInvestigation_Cooldown asserts that PersistInvestigation
// suppresses repeat calls for the same (trigger_service, root_service,
// root_operation) inside the configured cooldown window. Without this,
// a single stuck service produces one investigation insert every
// anomaly tick (default 10s) indefinitely.
//
// The counter exposed via InvestigationInsertCount() increments when
// the cooldown check passes, BEFORE the DB write — so the test is
// meaningful even when the test helper wires a nil repo. See the
// doc comment on InvestigationInsertCount for the exact semantics.
func TestPersistInvestigation_Cooldown(t *testing.T) {
	g := newTestGraphRAG(t)

	chains := []ErrorChainResult{{
		TraceID:   "tr",
		RootCause: &RootCauseInfo{Service: "orders", Operation: "op"},
	}}

	g.PersistInvestigation("orders", chains, nil)
	first := g.InvestigationInsertCount()
	if first == 0 {
		t.Fatalf("first PersistInvestigation should insert, got count=0")
	}

	g.PersistInvestigation("orders", chains, nil)
	second := g.InvestigationInsertCount()
	if second != first {
		t.Fatalf("second PersistInvestigation within cooldown should be suppressed; got %d new inserts", second-first)
	}

	chains2 := []ErrorChainResult{{
		TraceID:   "tr2",
		RootCause: &RootCauseInfo{Service: "payments", Operation: "op"},
	}}
	g.PersistInvestigation("payments", chains2, nil)
	third := g.InvestigationInsertCount()
	if third <= second {
		t.Fatalf("distinct service should bypass cooldown; got %d, want > %d", third, second)
	}
}
