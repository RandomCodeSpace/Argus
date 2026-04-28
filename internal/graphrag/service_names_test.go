package graphrag

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/RandomCodeSpace/otelcontext/internal/storage"
)

// TestServiceNames_IncludesDeepCalleesAndIsTenantScoped covers the bug that
// motivated this method: prior to ServiceNames, /api/metadata/services
// queried `SELECT DISTINCT service_name FROM traces`, which silently dropped
// any service that only ever appeared as a callee (deep in a fan-out) — its
// span lost the trace_id-uniqueness race and never made it into the traces
// table. ServiceNames reads from the in-memory ServiceStore where every
// span — root or child — registers via UpsertService.
func TestServiceNames_IncludesDeepCalleesAndIsTenantScoped(t *testing.T) {
	g := newTestGraphRAG(t)

	now := time.Now()
	mk := func(tenant, service, traceID, spanID, parentSpan string) storage.Span {
		return storage.Span{
			TenantID:      tenant,
			TraceID:       traceID,
			SpanID:        spanID,
			ParentSpanID:  parentSpan,
			OperationName: "/op",
			ServiceName:   service,
			Status:        "STATUS_CODE_OK",
			StartTime:     now,
			EndTime:       now.Add(time.Millisecond),
			Duration:      1000,
		}
	}

	// Tenant A: a 3-deep fan-out under one trace_id. order is the root,
	// payment is a child, shipping is a grandchild — exactly the pattern
	// where shipping previously got dropped from the dropdown.
	g.OnSpanIngested(mk("tenant-a", "order", "t-a-1", "root", ""))
	g.OnSpanIngested(mk("tenant-a", "payment", "t-a-1", "child", "root"))
	g.OnSpanIngested(mk("tenant-a", "shipping", "t-a-1", "grand", "child"))

	// Tenant B: a single root in a separate tenant. Must not leak into A.
	g.OnSpanIngested(mk("tenant-b", "audit", "t-b-1", "root", ""))

	ctxA := storage.WithTenantContext(context.Background(), "tenant-a")
	ctxB := storage.WithTenantContext(context.Background(), "tenant-b")

	// Async event workers — poll until both tenants have settled.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(g.ServiceNames(ctxA)) >= 3 && len(g.ServiceNames(ctxB)) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	gotA := g.ServiceNames(ctxA)
	wantA := []string{"order", "payment", "shipping"} // sorted ascending
	if !reflect.DeepEqual(gotA, wantA) {
		t.Fatalf("tenant-a ServiceNames = %v, want %v (sorted, includes deep callee)", gotA, wantA)
	}

	gotB := g.ServiceNames(ctxB)
	wantB := []string{"audit"}
	if !reflect.DeepEqual(gotB, wantB) {
		t.Fatalf("tenant-b ServiceNames = %v, want %v (no leak from tenant-a)", gotB, wantB)
	}
}

// TestServiceNames_EmptyOnColdStart asserts that a freshly-constructed
// GraphRAG with no ingested spans returns an empty slice (never nil),
// matching the JSON-encoder expectation of `[]` rather than `null` so the
// UI dropdown stays a valid array on first paint.
func TestServiceNames_EmptyOnColdStart(t *testing.T) {
	g := newTestGraphRAG(t)
	got := g.ServiceNames(context.Background())
	if got == nil {
		t.Fatal("ServiceNames returned nil; want empty slice for json `[]` round-trip")
	}
	if len(got) != 0 {
		t.Fatalf("ServiceNames on empty store = %v, want []", got)
	}
}
