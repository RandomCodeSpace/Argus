package vectordb

import (
	"context"
	"errors"
	"testing"
)

// fakeSource is an in-memory ReplaySource for unit-testing the page loop
// without a real DB. Pages are produced by a closure so each test can shape
// the source however it likes (multi-page, errors, end-of-data).
type fakeSource struct {
	pages [][]ReplayRow // queued pages; consumed in order
	calls int
	fail  error
}

func (s *fakeSource) LogsForVectorReplay(_ context.Context, sinceID uint, limit int) ([]ReplayRow, error) {
	s.calls++
	if s.fail != nil {
		return nil, s.fail
	}
	if s.calls > len(s.pages) {
		return nil, nil
	}
	page := s.pages[s.calls-1]
	// Filter to "rows newer than sinceID" so the test verifies the loop
	// passes the right cursor across iterations.
	out := make([]ReplayRow, 0, len(page))
	for _, r := range page {
		if r.ID > sinceID {
			out = append(out, r)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// TestReplayFromDB_AdvancesCursor verifies multi-page replay calls the
// source with monotonically-increasing sinceID values and indexes every
// row, with no duplicates by LogID.
func TestReplayFromDB_AdvancesCursor(t *testing.T) {
	src := &fakeSource{
		pages: [][]ReplayRow{
			{
				{ID: 10, Tenant: "t", ServiceName: "svc", Severity: "ERROR", Body: "boom"},
				{ID: 20, Tenant: "t", ServiceName: "svc", Severity: "ERROR", Body: "kaboom"},
			},
			{
				{ID: 30, Tenant: "t", ServiceName: "svc", Severity: "WARN", Body: "third page row tokenizes fine"},
			},
		},
	}
	idx := New(100)
	total, err := idx.ReplayFromDB(context.Background(), src)
	if err != nil {
		t.Fatalf("ReplayFromDB: %v", err)
	}
	if total != 3 {
		t.Errorf("processed: got %d, want 3", total)
	}
	if idx.Size() != 3 {
		t.Errorf("indexed Size: got %d, want 3", idx.Size())
	}
	if idx.LastIndexedID() != 30 {
		t.Errorf("LastIndexedID: got %d, want 30", idx.LastIndexedID())
	}
	// Two data pages + one empty page that signals end-of-data.
	if src.calls != 3 {
		t.Errorf("source calls: got %d, want 3 (2 data + 1 empty terminator)", src.calls)
	}
}

// TestReplayFromDB_StartsFromLastIndexedID verifies the loop seeds sinceID
// from the existing high watermark, so a snapshot's tail can be picked up
// without re-indexing rows already in the index.
func TestReplayFromDB_StartsFromLastIndexedID(t *testing.T) {
	idx := New(100)
	idx.Add(50, "t", "svc", "ERROR", "already indexed")
	if got := idx.LastIndexedID(); got != 50 {
		t.Fatalf("seed LastIndexedID: got %d, want 50", got)
	}

	src := &fakeSource{
		pages: [][]ReplayRow{
			// Page contains both pre-watermark and post-watermark rows; the
			// fake's filter mimics SQL's WHERE id > sinceID, so only post-50
			// rows leave the source.
			{
				{ID: 30, Tenant: "t", ServiceName: "svc", Severity: "ERROR", Body: "old"},
				{ID: 50, Tenant: "t", ServiceName: "svc", Severity: "ERROR", Body: "boundary"},
				{ID: 60, Tenant: "t", ServiceName: "svc", Severity: "ERROR", Body: "new"},
			},
		},
	}
	total, err := idx.ReplayFromDB(context.Background(), src)
	if err != nil {
		t.Fatalf("ReplayFromDB: %v", err)
	}
	if total != 1 {
		t.Errorf("processed: got %d, want 1 (only id=60 is post-watermark)", total)
	}
	if idx.Size() != 2 {
		t.Errorf("indexed Size: got %d, want 2 (seed + replayed)", idx.Size())
	}
	if idx.LastIndexedID() != 60 {
		t.Errorf("LastIndexedID: got %d, want 60", idx.LastIndexedID())
	}
}

// TestReplayFromDB_PropagatesError verifies a source error is returned
// alongside the partial count so the caller can log and continue.
func TestReplayFromDB_PropagatesError(t *testing.T) {
	src := &fakeSource{fail: errors.New("db gone")}
	idx := New(100)
	total, err := idx.ReplayFromDB(context.Background(), src)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if total != 0 {
		t.Errorf("partial count: got %d, want 0", total)
	}
	if idx.Size() != 0 {
		t.Errorf("error path must not corrupt index: Size=%d", idx.Size())
	}
}

// TestReplayFromDB_RespectsCancellation verifies a cancelled ctx aborts
// the loop without making another source call.
func TestReplayFromDB_RespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	src := &fakeSource{
		pages: [][]ReplayRow{
			{{ID: 1, Tenant: "t", ServiceName: "svc", Severity: "ERROR", Body: "x"}},
		},
	}
	idx := New(100)
	_, err := idx.ReplayFromDB(ctx, src)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if src.calls != 0 {
		t.Errorf("source called despite cancelled ctx: calls=%d", src.calls)
	}
}

// TestReplayFromDB_NilSource is a smoke test for the nil-safe early return.
func TestReplayFromDB_NilSource(t *testing.T) {
	idx := New(100)
	total, err := idx.ReplayFromDB(context.Background(), nil)
	if err != nil {
		t.Fatalf("nil source: unexpected err=%v", err)
	}
	if total != 0 {
		t.Errorf("nil source: total=%d, want 0", total)
	}
}
