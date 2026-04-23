package graphrag

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// Preprocess edge cases — builds confidence that masks don't over-/under-match.
func TestDrain_Preprocess_EdgeCases(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		mustHave []string
		mustNot  []string
	}{
		{
			name:     "ipv4 with port",
			in:       "dial tcp 10.0.0.1:5432: connection refused",
			mustHave: []string{"<IP>", "connection", "refused"},
			mustNot:  []string{"10.0.0.1", "5432"},
		},
		{
			name:     "ipv4 without port",
			in:       "client 192.168.1.100 connected",
			mustHave: []string{"<IP>", "client", "connected"},
			mustNot:  []string{"192.168"},
		},
		{
			name:     "uuid lowercase and uppercase",
			in:       "user 550e8400-e29b-41d4-a716-446655440000 session 5F9E8400-E29B-41D4-A716-446655440000",
			mustHave: []string{"<UUID>"},
			mustNot:  []string{"550e8400", "5F9E8400", "446655440000"},
		},
		{
			name:     "hex 0x prefix",
			in:       "addr 0xdeadbeef01 pointer 0xCAFE",
			mustHave: []string{"<HEX>"},
			mustNot:  []string{"0xdeadbeef01", "0xCAFE"},
		},
		{
			name:     "long hex (session IDs)",
			in:       "session abcdef0123456789abcdef0123456789 created",
			mustHave: []string{"<HEX>", "session", "created"},
			mustNot:  []string{"abcdef0123456789"},
		},
		{
			name:     "email addresses",
			in:       "notify alice@example.com and bob.smith+tag@sub.example.co.uk",
			mustHave: []string{"<EMAIL>", "notify"},
			mustNot:  []string{"alice@example.com"},
		},
		{
			name:     "negative and decimal numbers",
			in:       "duration -1.5 seconds ratio 0.95 count -42",
			mustHave: []string{"<NUM>", "duration", "seconds", "ratio", "count"},
			mustNot:  []string{"-1.5", "0.95", "-42"},
		},
		{
			name:     "iso8601 timestamp",
			in:       "event at 2026-04-17T06:11:50Z happened",
			mustHave: []string{"<TS>", "event", "happened"},
			mustNot:  []string{"2026-04-17"},
		},
		{
			name:     "unix ms timestamp",
			in:       "recorded at 1713333710000 done",
			mustHave: []string{"<TS>", "recorded", "done"},
			mustNot:  []string{"1713333710000"},
		},
		{
			name:     "port without ip (bare)",
			in:       "listening on :8080",
			mustHave: []string{"<NUM>", "listening"},
			mustNot:  []string{"8080"},
		},
		{
			name: "empty string",
			in:   "",
		},
		{
			name: "whitespace only",
			in:   "   \t  ",
		},
		{
			name:     "unicode / emoji passthrough",
			in:       "unicorn 🦄 processed успешно",
			mustHave: []string{"processed"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Preprocess(c.in)
			for _, m := range c.mustHave {
				if !strings.Contains(got, m) {
					t.Fatalf("preprocess(%q)=%q missing %q", c.in, got, m)
				}
			}
			for _, m := range c.mustNot {
				if strings.Contains(got, m) {
					t.Fatalf("preprocess(%q)=%q should not contain %q", c.in, got, m)
				}
			}
		})
	}
}

// Logs that differ only by variable tokens must converge to ONE template.
func TestDrain_SameShapeConvergesToOneTemplate(t *testing.T) {
	d := NewDrain()
	lines := []string{
		"User 1 logged in from 10.0.0.1 at 2026-04-17T06:00:00Z",
		"User 42 logged in from 10.0.0.2 at 2026-04-17T06:00:01Z",
		"User 9999 logged in from 172.16.0.5 at 2026-04-17T06:00:05Z",
		"User -1 logged in from 192.168.100.200 at 2026-04-17T06:00:10Z",
	}
	ids := map[uint64]int{}
	for _, l := range lines {
		tpl := d.Match(l, time.Now())
		ids[tpl.ID]++
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 template, got %d", len(ids))
	}
	tpls := d.Templates()
	if len(tpls) != 1 {
		t.Fatalf("Templates() = %d want 1", len(tpls))
	}
	if tpls[0].Count != len(lines) {
		t.Fatalf("template Count=%d want %d", tpls[0].Count, len(lines))
	}
}

// Distinct-shape logs must produce distinct templates.
func TestDrain_DistinctShapesDistinctTemplates(t *testing.T) {
	d := NewDrain()
	d.Match("connection refused from 10.0.0.1", time.Now())
	d.Match("user login failed: bad password", time.Now())
	d.Match("cache miss for key xyz", time.Now())

	if got := len(d.Templates()); got != 3 {
		t.Fatalf("expected 3 distinct templates, got %d", got)
	}
}

// Template IDs are stable across restarts for identical token sequences.
func TestDrain_TemplateIDStability(t *testing.T) {
	d1 := NewDrain()
	d2 := NewDrain()
	line := "processing request 123 for user 456"

	tpl1 := d1.Match(line, time.Now())
	tpl2 := d2.Match(line, time.Now())
	if tpl1.ID != tpl2.ID {
		t.Fatalf("template IDs not stable across instances: %d vs %d", tpl1.ID, tpl2.ID)
	}
}

// Empty input must not crash and should return a non-nil template (or nil — either way no panic).
func TestDrain_EmptyAndSingleToken(t *testing.T) {
	d := NewDrain()

	// Must not panic.
	_ = d.Match("", time.Now())
	_ = d.Match("single", time.Now())
	_ = d.Match("   ", time.Now())
	_ = d.Match("\n\t", time.Now())
}

// Concurrent ingestion must be race-free and produce a stable template population.
func TestDrain_ConcurrentIngest_NoRace(t *testing.T) {
	d := NewDrain()
	var wg sync.WaitGroup
	shapes := []string{
		"User %d logged in from 10.0.%d.%d",
		"cache miss for key %d in bucket %d",
		"request %d took %d ms",
	}
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				shape := shapes[i%len(shapes)]
				line := fmt.Sprintf(shape, i, (i+seed)%256, (seed+i*3)%256)
				d.Match(line, time.Now())
			}
		}(w)
	}
	wg.Wait()

	// Each shape must converge to a single template.
	tpls := d.Templates()
	if len(tpls) > len(shapes)*2 { // generous slack for depth quirks
		t.Fatalf("too many templates: got %d for %d shapes", len(tpls), len(shapes))
	}
	totalCount := 0
	for _, tpl := range tpls {
		totalCount += tpl.Count
	}
	if totalCount != 8*500 {
		t.Fatalf("total counts mismatch: got %d want %d", totalCount, 8*500)
	}
}

// Snapshot / restore round-trip preserves template set.
func TestDrain_SnapshotRestore(t *testing.T) {
	d1 := NewDrain()
	for i := 0; i < 50; i++ {
		d1.Match(fmt.Sprintf("event %d on host %d.%d.%d.%d", i, i%256, 10, 20, 30), time.Now())
	}
	snap := d1.Templates()

	d2 := NewDrain()
	d2.LoadTemplates(snap)

	// Match an identical line — must find an existing template.
	tpl := d2.Match("event 999 on host 255.255.255.255", time.Now())
	found := false
	for _, s := range snap {
		if s.ID == tpl.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("post-restore match produced a template not in the snapshot")
	}
}

// LRU eviction bounds the template count and evicts the oldest-seen template.
func TestDrain_LRUEviction_RespectsCap(t *testing.T) {
	d := NewDrain(WithMaxTemplates(5))
	for i := 0; i < 20; i++ {
		// Use distinctly-shaped (different token counts) lines to force new templates.
		tokens := make([]string, i+1)
		for j := range tokens {
			tokens[j] = fmt.Sprintf("tok%d_%d", i, j)
		}
		d.Match(strings.Join(tokens, " "), time.Now())
	}
	if got := len(d.Templates()); got > 5 {
		t.Fatalf("LRU cap violated: %d templates > max 5", got)
	}
}

// Extremely long lines must not break preprocess or matching.
func TestDrain_PathologicalLongLine(t *testing.T) {
	long := strings.Repeat("word ", 10_000)
	d := NewDrain()
	tpl := d.Match(long, time.Now())
	if tpl == nil {
		t.Fatal("nil template for long line")
	}
}

// Merging: template tokens that were concrete must become wildcards when they differ.
func TestDrain_TemplateMergePromotesToWildcard(t *testing.T) {
	d := NewDrain()
	d.Match("status OK latency 10 ms", time.Now())
	d.Match("status OK latency 50 ms", time.Now())
	d.Match("status OK latency 150 ms", time.Now())

	tpls := d.Templates()
	if len(tpls) != 1 {
		t.Fatalf("expected 1 template, got %d", len(tpls))
	}
	joined := strings.Join(tpls[0].Tokens, " ")
	if !strings.Contains(joined, "<*>") && !strings.Contains(joined, "<NUM>") {
		t.Fatalf("expected wildcard/num in merged template, got %q", joined)
	}
	// Common literals must be preserved.
	if !strings.Contains(joined, "status") || !strings.Contains(joined, "latency") {
		t.Fatalf("literal tokens lost: %q", joined)
	}
}

// Ensure Count is monotonic across many calls for the same template.
func TestDrain_CountIsMonotonic(t *testing.T) {
	d := NewDrain()
	const N = 1000
	var lastID uint64
	for i := 0; i < N; i++ {
		tpl := d.Match(fmt.Sprintf("event %d happened", i), time.Now())
		if i == 0 {
			lastID = tpl.ID
		} else if tpl.ID != lastID {
			t.Fatalf("template ID changed mid-stream: %d -> %d at i=%d", lastID, tpl.ID, i)
		}
	}
	tpls := d.Templates()
	if len(tpls) != 1 || tpls[0].Count != int(N) {
		t.Fatalf("want 1 template with Count=%d; got %+v", N, tpls)
	}
}

// FirstSeen / LastSeen timestamps must be updated correctly.
func TestDrain_TimestampsAccurate(t *testing.T) {
	d := NewDrain()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	d.Match("event A", t0)
	d.Match("event A", t1)

	tpls := d.Templates()
	if len(tpls) != 1 {
		t.Fatalf("want 1 template, got %d", len(tpls))
	}
	if !tpls[0].FirstSeen.Equal(t0) {
		t.Fatalf("FirstSeen=%v want %v", tpls[0].FirstSeen, t0)
	}
	if !tpls[0].LastSeen.Equal(t1) {
		t.Fatalf("LastSeen=%v want %v", tpls[0].LastSeen, t1)
	}
}

// BenchmarkDrainMatch_AtCapacity fills Drain to its template cap and measures
// Match() throughput under sustained eviction pressure. With the container/list
// LRU and leaf back-pointer the eviction path is O(1) tree lookup plus O(M)
// leaf-slice removal where M is small, versus the prior O(N) map+tree scan.
func BenchmarkDrainMatch_AtCapacity(b *testing.B) {
	const cap = 1000
	// Raise WithMaxChildren so the benchmark-only shape explosion is not
	// collapsed by the prefix-tree's per-node wildcard cap. Also bump
	// similarity threshold to near-1 so near-duplicate shapes don't merge.
	d := NewDrain(
		WithMaxTemplates(cap),
		WithMaxChildren(4096),
		WithSimilarityThreshold(0.95),
	)
	// Preprocess masks digits to <NUM>, so differentiation must be textual.
	tag := func(i int) string {
		if i == 0 {
			return "aa"
		}
		out := make([]byte, 0, 6)
		for i > 0 {
			out = append(out, 'a'+byte(i%26))
			i /= 26
		}
		return string(out)
	}
	// Prime to full capacity with distinct shapes: vary both the first token
	// (prefix) and token count (length layer) so each line lands in a unique
	// leaf and no two shapes are similar enough to merge.
	for i := 0; i < cap; i++ {
		t1 := tag(i)
		t2 := tag(i + 1)
		t3 := tag(i * 7)
		t4 := tag(i * 13)
		line := fmt.Sprintf("%s %s %s %s end", t1, t2, t3, t4)
		d.Match(line, time.Now())
	}
	if d.Len() < cap {
		b.Fatalf("prime failed: Len=%d want >=%d", d.Len(), cap)
	}
	b.ReportAllocs()
	b.ResetTimer()
	// Each iteration forces an eviction by introducing a brand-new template.
	for i := 0; i < b.N; i++ {
		t1 := tag(i + cap)
		t2 := tag(i + cap + 1)
		t3 := tag((i + cap) * 7)
		t4 := tag((i + cap) * 13)
		line := fmt.Sprintf("%s %s %s %s hot", t1, t2, t3, t4)
		d.Match(line, time.Now())
	}
}

// BenchmarkDrainMatch_Parallel exercises Match under contention to document
// the effect of running Preprocess/tokenize outside the write lock. 8
// goroutines each feed distinct shapes so the prefix tree gets populated but
// the CPU-bound regex work does not serialize behind d.mu.
func BenchmarkDrainMatch_Parallel(b *testing.B) {
	d := NewDrain()
	shapes := []string{
		"User %d logged in from 10.0.%d.%d at 2026-04-17T06:00:00Z",
		"cache miss for key item-%d in bucket %d shard %d",
		"request %d took %d ms for tenant %d",
		"dial tcp 10.0.%d.%d:%d connection refused after %d attempts",
		"processed event %d of type alpha by worker %d partition %d",
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.SetParallelism(8)
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			shape := shapes[i%len(shapes)]
			line := fmt.Sprintf(shape, i, (i*7)%256, (i*13)%256, i%1024)
			d.Match(line, time.Now())
			i++
		}
	})
}
