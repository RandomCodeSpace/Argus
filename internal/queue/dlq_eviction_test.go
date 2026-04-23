package queue

import (
	"testing"
	"time"
)

// TestDLQ_EvictionIncrementsCounters verifies that when the DLQ is bounded by
// MaxFiles and exceeded, the eviction atomic counters climb. The underlying
// prometheus counters are exercised indirectly via a nil metricsTel path
// (no wiring) and explicitly in main.go integration.
func TestDLQ_EvictionIncrementsCounters(t *testing.T) {
	dir := t.TempDir()

	// replayFn is a no-op — we care about eviction during Enqueue, not replay.
	noReplay := func(_ []byte) error { return nil }

	// Cap at 2 files with a long replay interval so the worker doesn't interfere.
	dlq, err := NewDLQWithLimits(dir, time.Hour, noReplay, 2, 0, 0)
	if err != nil {
		t.Fatalf("NewDLQWithLimits: %v", err)
	}
	defer dlq.Stop()

	payload := map[string]any{"type": "spans", "data": []string{}}
	for i := 0; i < 3; i++ {
		if err := dlq.Enqueue(payload); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
		// Tiny sleep to guarantee distinct nanosecond prefixes for FIFO order.
		time.Sleep(5 * time.Millisecond)
	}

	if got := dlq.EvictedCount(); got == 0 {
		t.Fatalf("expected eviction count > 0, got %d", got)
	}
	if got := dlq.EvictedBytesCount(); got == 0 {
		t.Fatalf("expected evicted bytes > 0, got %d", got)
	}
}
