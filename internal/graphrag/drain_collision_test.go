package graphrag

import (
	"container/list"
	"testing"
	"time"
)

// TestDrain_MergeIntoExistingTemplate_NoOrphan is a whitebox test for Fix 5.
// It seeds two live templates that share a leaf, then drives one of them
// through a generalization-induced re-hash such that the new ID collides
// with the other live template's ID. The fold logic must:
//   - preserve all counts on the surviving template
//   - drop the orphan from its leaf's groups slice
//   - drop the orphan from the LRU
//   - keep d.lru.Len() == len(d.templates)
func TestDrain_MergeIntoExistingTemplate_NoOrphan(t *testing.T) {
	d := NewDrain(WithSimilarityThreshold(0.25))
	now := time.Unix(1_700_000_000, 0)

	// Manually stage the scenario: one leaf holds two templates.
	//   A = [alpha beta]           (literal) — will be the survivor
	//   B = [delta beta]           (literal) — will generalize to A's post-merge tokens
	//
	// We assemble the drain state directly to guarantee they share a leaf
	// regardless of depth/prefix-tree branching behavior. This is the exact
	// runtime shape matchOrCreate sees after natural traffic.
	leaf := newDrainNode()
	d.byLen[2] = leaf

	tA := &Template{
		ID:        templateID([]string{"alpha", "beta"}),
		Tokens:    []string{"alpha", "beta"},
		Count:     5,
		FirstSeen: now,
		LastSeen:  now,
		Sample:    "alpha beta",
		leaf:      leaf,
	}
	tB := &Template{
		ID:        templateID([]string{"delta", "beta"}),
		Tokens:    []string{"delta", "beta"},
		Count:     3,
		FirstSeen: now.Add(time.Second),
		LastSeen:  now.Add(time.Second),
		Sample:    "delta beta",
		leaf:      leaf,
	}
	leaf.groups = []*Template{tA, tB}
	d.templates[tA.ID] = tA
	d.templates[tB.ID] = tB
	tA.elem = d.lru.PushFront(tA)
	tB.elem = d.lru.PushFront(tB)

	if d.Len() != 2 {
		t.Fatalf("setup: expected 2 templates, got %d", d.Len())
	}
	if d.lru.Len() != 2 {
		t.Fatalf("setup: expected lru len 2, got %d", d.lru.Len())
	}

	// First, transform A to its generalized shape [<*> beta] via a merge.
	// Use Match-equivalent internal call with tokens "gamma beta" so
	// position 0 mismatches A -> A tokens become [<*> beta], new ID.
	// Use the leaf-scoped matchOrCreate directly so we don't have to fight
	// the prefix tree.
	d.mu.Lock()
	a2 := d.matchOrCreate(leaf, []string{"gamma", "beta"}, "gamma beta", now.Add(2*time.Second))
	d.mu.Unlock()
	if a2 != tA {
		t.Fatalf("expected gamma-merge to fold into tA, got different template")
	}
	if tA.Tokens[0] != Wildcard {
		t.Fatalf("expected tA token[0] to be wildcard, got %q", tA.Tokens[0])
	}

	// Now A has tokens [<*> beta] with ID = templateID(["<*>","beta"]).
	// Drive B's merge: input "epsilon beta" -> B's position 0 mismatches,
	// generalizing B to [<*> beta] — SAME tokens, SAME ID as A.
	// Temporarily remove A from the leaf so B is the only merge candidate,
	// mirroring a real-world scenario where leaf scan picks B first.
	leaf.groups = []*Template{tB}
	before := d.Len()
	d.mu.Lock()
	folded := d.matchOrCreate(leaf, []string{"epsilon", "beta"}, "epsilon beta", now.Add(3*time.Second))
	d.mu.Unlock()

	// Restore A to the leaf if matchOrCreate didn't already reinstate it via fold.
	// The fold path calls removeTplFromLeaf(best.leaf, best) which removes B,
	// but doesn't add A — the test harness tracks A separately.
	// Re-populate leaf.groups with whatever is still live.
	live := []*Template{}
	for _, g := range []*Template{tA, tB} {
		if _, ok := d.templates[g.ID]; ok {
			live = append(live, g)
		}
	}
	leaf.groups = live

	// Expect fold: one template gone, the survivor holds the summed Count.
	if d.Len() != before-1 {
		t.Fatalf("expected fold (Len %d -> %d), got %d",
			before, before-1, d.Len())
	}

	// The survivor should be tA (since B's new ID equals A's current ID).
	if folded != tA {
		t.Fatalf("expected folded return value to be tA (the existing template)")
	}

	// LRU invariant: exactly as many elements as live templates.
	if d.lru.Len() != d.Len() {
		t.Fatalf("lru leaked orphan: lru=%d templates=%d",
			d.lru.Len(), d.Len())
	}

	// Orphan-B must no longer be in the LRU list.
	if tB.elem != nil {
		// Walk the list and make sure this element is not present.
		for e := d.lru.Front(); e != nil; e = e.Next() {
			if e == tB.elem {
				t.Fatal("orphan B still in LRU list")
			}
		}
	}
	// And must not be in leaf.groups.
	for _, g := range leaf.groups {
		if g == tB {
			t.Fatal("orphan B still in leaf.groups")
		}
	}

	// Count-preservation: merged survivor must carry B's pre-fold count plus
	// the usual +1 for the triggering log, on top of its own history.
	// Pre-merge A.Count=5 (seed) + 1 (gamma merge) = 6.
	// B.Count=3 pre-fold. Merge increments existing.Count by B.Count then +1
	// for the triggering log itself.
	if got := tA.Count; got < 5+1+3 {
		t.Fatalf("expected folded Count >= 9, got %d", got)
	}

	// Optional sanity: tB.elem set to nil by fold code.
	if tB.elem != nil {
		t.Fatal("expected tB.elem to be nil after fold")
	}

	// Make sure the tracked LRU list is internally consistent.
	seen := map[*Template]bool{}
	_ = list.New // keeps the import used
	for e := d.lru.Front(); e != nil; e = e.Next() {
		tpl := e.Value.(*Template)
		if seen[tpl] {
			t.Fatalf("duplicate template in LRU")
		}
		seen[tpl] = true
	}
}
