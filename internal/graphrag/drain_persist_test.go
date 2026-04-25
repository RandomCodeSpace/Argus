package graphrag

import (
	"fmt"
	"testing"
	"time"

	"github.com/RandomCodeSpace/otelcontext/internal/storage"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newTestDrainDB stands up an in-memory SQLite DB with the drain_templates
// table migrated. Kept local to this file to avoid coupling to storage test
// helpers (which live in a _test-only scope of another package).
func newTestDrainDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&DrainTemplateRow{}); err != nil {
		t.Fatalf("migrate DrainTemplateRow: %v", err)
	}
	return db
}

// TestDrainPersistence_RoundTrip feeds N logs into one Drain instance, saves
// templates, reloads into a fresh Drain, and asserts identical lines match
// the restored template IDs.
func TestDrainPersistence_RoundTrip(t *testing.T) {
	db := newTestDrainDB(t)
	d1 := NewDrain()

	// Generate 50 distinct shapes. Numeric values are masked to <NUM> by
	// Preprocess, so shape differentiators must be textual.
	lines := make([]string, 0, 50)
	idMap := make(map[string]uint64, 50)
	ts := time.Now()
	for i := 0; i < 50; i++ {
		line := fmt.Sprintf("shape %s event processed token alpha", fmt.Sprintf("kind-%c%c", 'a'+byte(i/10), 'a'+byte(i%10)))
		lines = append(lines, line)
		tpl := d1.Match(line, ts)
		if tpl == nil {
			t.Fatalf("Match returned nil for %q", line)
		}
		idMap[line] = tpl.ID
	}
	if d1.Len() != 50 {
		t.Fatalf("d1.Len()=%d want 50", d1.Len())
	}

	// Persist.
	if err := SaveDrainTemplates(db, storage.DefaultTenantID, d1.Templates()); err != nil {
		t.Fatalf("SaveDrainTemplates: %v", err)
	}

	// Reload.
	loaded, err := LoadDrainTemplates(db, storage.DefaultTenantID)
	if err != nil {
		t.Fatalf("LoadDrainTemplates: %v", err)
	}
	if len(loaded) != 50 {
		t.Fatalf("loaded=%d want 50", len(loaded))
	}

	// Rebuild fresh Drain.
	d2 := NewDrain()
	d2.LoadTemplates(loaded)
	if d2.Len() != 50 {
		t.Fatalf("d2.Len()=%d want 50 after LoadTemplates", d2.Len())
	}

	// Matching an identical log line should hit the same template ID.
	for _, line := range lines {
		got := d2.Match(line, ts)
		if got == nil {
			t.Fatalf("restored Match returned nil for %q", line)
		}
		if got.ID != idMap[line] {
			t.Fatalf("line %q: restored ID=%d want %d", line, got.ID, idMap[line])
		}
	}
}

// TestDrainPersistence_Upsert saves twice with mutated Count and LastSeen,
// asserts row count does not double and mutable fields are updated.
func TestDrainPersistence_Upsert(t *testing.T) {
	db := newTestDrainDB(t)
	d := NewDrain()
	t0 := time.Now().Truncate(time.Second)
	for i := 0; i < 10; i++ {
		d.Match(fmt.Sprintf("upsert kind %c event", 'a'+byte(i)), t0)
	}
	if err := SaveDrainTemplates(db, storage.DefaultTenantID, d.Templates()); err != nil {
		t.Fatalf("first save: %v", err)
	}

	var cnt1 int64
	if err := db.Model(&DrainTemplateRow{}).Count(&cnt1).Error; err != nil {
		t.Fatalf("count after first save: %v", err)
	}
	if cnt1 != 10 {
		t.Fatalf("after first save got %d rows, want 10", cnt1)
	}

	// Re-feed the same shapes with a later timestamp to bump Count + LastSeen.
	t1 := t0.Add(1 * time.Hour)
	for i := 0; i < 10; i++ {
		d.Match(fmt.Sprintf("upsert kind %c event", 'a'+byte(i)), t1)
	}
	if err := SaveDrainTemplates(db, storage.DefaultTenantID, d.Templates()); err != nil {
		t.Fatalf("second save: %v", err)
	}

	var cnt2 int64
	if err := db.Model(&DrainTemplateRow{}).Count(&cnt2).Error; err != nil {
		t.Fatalf("count after second save: %v", err)
	}
	if cnt2 != 10 {
		t.Fatalf("after second save got %d rows, want 10 (upsert should not duplicate)", cnt2)
	}

	// Every row should have count >= 2 and LastSeen >= t1.
	var rows []DrainTemplateRow
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	for _, r := range rows {
		if r.Count < 2 {
			t.Fatalf("row id=%d Count=%d want >=2 after upsert", r.ID, r.Count)
		}
		if r.LastSeen.Before(t1) {
			t.Fatalf("row id=%d LastSeen=%v want >=%v", r.ID, r.LastSeen, t1)
		}
	}
}

// TestDrainPersistence_EmptyDB verifies that loading from an empty table
// returns an empty slice and nil error (fresh-install path).
func TestDrainPersistence_EmptyDB(t *testing.T) {
	db := newTestDrainDB(t)
	tpls, err := LoadDrainTemplates(db, storage.DefaultTenantID)
	if err != nil {
		t.Fatalf("LoadDrainTemplates on empty table: %v", err)
	}
	if len(tpls) != 0 {
		t.Fatalf("empty table returned %d templates, want 0", len(tpls))
	}

	// Calling with nil DB is also safe.
	tpls, err = LoadDrainTemplates(nil, storage.DefaultTenantID)
	if err != nil {
		t.Fatalf("LoadDrainTemplates(nil): %v", err)
	}
	if len(tpls) != 0 {
		t.Fatalf("nil DB returned %d templates, want 0", len(tpls))
	}

	// Saving an empty slice is a no-op (no error, no rows).
	if err := SaveDrainTemplates(db, storage.DefaultTenantID, nil); err != nil {
		t.Fatalf("SaveDrainTemplates(nil): %v", err)
	}
	var cnt int64
	if err := db.Model(&DrainTemplateRow{}).Count(&cnt).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("after save-nil got %d rows, want 0", cnt)
	}
}
