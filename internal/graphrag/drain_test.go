package graphrag

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPreprocessMasking(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string // substrings that must appear (or not) in the masked output
		not  []string
	}{
		{
			name: "ipv4",
			in:   "connected to 10.0.0.1:8080 successfully",
			want: []string{"<IP>"},
			not:  []string{"10.0.0.1"},
		},
		{
			name: "uuid",
			in:   "trace 550e8400-e29b-41d4-a716-446655440000 done",
			want: []string{"<UUID>"},
			not:  []string{"550e8400"},
		},
		{
			name: "hex 0x",
			in:   "pointer 0xdeadbeef",
			want: []string{"<HEX>"},
			not:  []string{"0xdeadbeef"},
		},
		{
			name: "long hex",
			in:   "sha256 abcdef0123456789abcdef0123456789",
			want: []string{"<HEX>"},
		},
		{
			name: "integer",
			in:   "processed 4711 records",
			want: []string{"<NUM>"},
			not:  []string{"4711"},
		},
		{
			name: "iso timestamp",
			in:   "event at 2025-01-02T03:04:05Z completed",
			want: []string{"<TS>"},
			not:  []string{"2025-01-02"},
		},
		{
			name: "email",
			in:   "notify alice@example.com now",
			want: []string{"<EMAIL>"},
			not:  []string{"alice@example.com"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Preprocess(c.in)
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Errorf("Preprocess(%q) = %q; want contains %q", c.in, got, w)
				}
			}
			for _, w := range c.not {
				if strings.Contains(got, w) {
					t.Errorf("Preprocess(%q) = %q; must not contain %q", c.in, got, w)
				}
			}
		})
	}
}

func TestSameShapeConverges(t *testing.T) {
	d := NewDrain()
	ts := time.Now()
	// Same prefix path (first `depth` tokens identical), differing tail.
	// Drain routes by prefix tokens, then merges tail mismatches into Wildcard.
	// Numbers are masked to <NUM> by preprocess, so these share the shape
	// "Request <NUM> completed in <NUM>ms" (<NUM> literal at tree level).
	t1 := d.Match("Request 42 completed in 120ms", ts)
	t2 := d.Match("Request 7 completed in 340ms", ts)
	if t1 == nil || t2 == nil {
		t.Fatal("nil template returned")
	}
	if t1.ID != t2.ID {
		t.Fatalf("same-shape logs produced different templates: %v vs %v", t1.Tokens, t2.Tokens)
	}
	if d.Len() != 1 {
		t.Errorf("expected 1 template, got %d", d.Len())
	}

	// Now test tail-position generalization: identical prefix path, varying
	// tail tokens beyond depth. With depth=4 and 6-token log, position 5
	// varies and should become Wildcard after two examples.
	d2 := NewDrain()
	a := d2.Match("GET /api/users 200 OK in fastpath", ts)
	b := d2.Match("GET /api/users 200 OK in slowpath", ts)
	if a.ID != b.ID {
		t.Fatalf("tail-variant logs not merged: %v vs %v", a.Tokens, b.Tokens)
	}
	joined := strings.Join(b.Tokens, " ")
	if !strings.Contains(joined, Wildcard) {
		t.Errorf("expected wildcard in merged tail template, got %q", joined)
	}
}

func TestDifferentShapesDistinct(t *testing.T) {
	d := NewDrain()
	ts := time.Now()
	a := d.Match("User alice login from host1", ts)
	b := d.Match("Database connection failed: timeout after 30s", ts)
	if a.ID == b.ID {
		t.Fatalf("different-shape logs collapsed to same template: %v", a.Tokens)
	}
	if d.Len() != 2 {
		t.Errorf("expected 2 templates, got %d", d.Len())
	}
}

func TestTemplateIDStability(t *testing.T) {
	tokens := []string{"User", Wildcard, "login", "from", Wildcard}
	id1 := templateID(tokens)
	id2 := templateID(tokens)
	if id1 != id2 {
		t.Errorf("templateID unstable: %d vs %d", id1, id2)
	}
	// Different token order → different ID.
	other := []string{"login", "User", Wildcard, "from", Wildcard}
	if templateID(other) == id1 {
		t.Error("templateID collision on different token order")
	}
}

func TestConcurrentMatchRaceFree(t *testing.T) {
	d := NewDrain()
	var wg sync.WaitGroup
	workers := 8
	per := 500
	ts := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(wid int) {
			defer wg.Done()
			for i := 0; i < per; i++ {
				line := fmt.Sprintf("worker %d processed request id=%d from 10.0.0.%d", wid, i, i%250)
				if d.Match(line, ts) == nil {
					t.Errorf("nil template")
					return
				}
			}
		}(w)
	}
	wg.Wait()
	if d.Len() == 0 {
		t.Error("expected at least one template after concurrent matches")
	}
	// Snapshot should also be race-free and non-empty.
	if got := d.Templates(); len(got) == 0 {
		t.Error("Templates() returned empty snapshot")
	}
}

func TestLRUEviction(t *testing.T) {
	d := NewDrain(WithMaxTemplates(3))
	base := time.Now()
	// Insert 5 clearly distinct templates, each older than the next.
	lines := []string{
		"alpha server ready",
		"beta cache ready",
		"gamma queue ready",
		"delta worker ready",
		"epsilon router ready",
	}
	for i, line := range lines {
		d.Match(line, base.Add(time.Duration(i)*time.Second))
	}
	if d.Len() > 3 {
		t.Errorf("LRU cap exceeded: %d templates, max 3", d.Len())
	}
	// Oldest ("alpha") must have been evicted.
	for _, tpl := range d.Templates() {
		if strings.Contains(tpl.Sample, "alpha") {
			t.Errorf("expected oldest template to be evicted, still present: %+v", tpl)
		}
	}
}

func TestSnapshotAndLoad(t *testing.T) {
	d := NewDrain()
	ts := time.Now()
	d.Match("Request 1 completed in 10ms", ts)
	d.Match("Request 2 completed in 20ms", ts) // merges with first
	d.Match("Database connection failed now", ts)

	snap := d.Templates()
	if len(snap) == 0 {
		t.Fatal("empty snapshot")
	}

	d2 := NewDrain()
	d2.LoadTemplates(snap)
	if d2.Len() != len(snap) {
		t.Errorf("Load mismatch: got %d, want %d", d2.Len(), len(snap))
	}
	// After reload, a new same-shape line should merge (no new template).
	before := d2.Len()
	d2.Match("Request 99 completed in 500ms", ts)
	if d2.Len() != before {
		t.Errorf("reloaded drain did not reuse template; before=%d after=%d", before, d2.Len())
	}
}

func TestMatchWildcardPreserved(t *testing.T) {
	d := NewDrain()
	ts := time.Now()
	// Use logs with >=6 literal tokens so position 5 (beyond depth=4) is
	// in the leaf similarity stage, where wildcards are introduced and
	// must stick across subsequent matches.
	d.Match("GET api users list via fastpath handler", ts)
	d.Match("GET api users list via slowpath handler", ts) // -> position 5 becomes Wildcard
	tpl := d.Match("GET api users list via newpath handler", ts)
	if tpl == nil {
		t.Fatal("nil template")
	}
	if tpl.Tokens[5] != Wildcard {
		t.Errorf("wildcard overwritten at position 5: %v", tpl.Tokens)
	}
}

func TestEmptyInput(t *testing.T) {
	d := NewDrain()
	if tpl := d.Match("", time.Now()); tpl != nil {
		t.Errorf("expected nil for empty input, got %+v", tpl)
	}
	if tpl := d.Match("   \t\n", time.Now()); tpl != nil {
		t.Errorf("expected nil for whitespace-only input, got %+v", tpl)
	}
}
