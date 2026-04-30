package vectordb

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestSnapshotRoundTrip verifies an encoded snapshot decodes back to the
// same logical state across all populated fields.
func TestSnapshotRoundTrip(t *testing.T) {
	in := Snapshot{
		LastIndexedID: 42,
		MaxSize:       1000,
		Docs: []LogVector{
			{LogID: 1, Tenant: "acme", ServiceName: "api", Severity: "ERROR", Body: "panic at startup", Vec: map[string]float64{"panic": 0.5, "startup": 0.5}},
			{LogID: 2, Tenant: "globex", ServiceName: "db", Severity: "WARN", Body: "timeout connecting", Vec: map[string]float64{"timeout": 1.0}},
		},
		IDF:       map[string]float64{"panic": 1.5, "startup": 1.0, "timeout": 1.2},
		WrittenAt: 1714464000,
	}
	var buf bytes.Buffer
	if err := EncodeSnapshot(&buf, in); err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := DecodeSnapshot(&buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.LastIndexedID != in.LastIndexedID {
		t.Errorf("LastIndexedID: got %d, want %d", out.LastIndexedID, in.LastIndexedID)
	}
	if out.MaxSize != in.MaxSize {
		t.Errorf("MaxSize: got %d, want %d", out.MaxSize, in.MaxSize)
	}
	if len(out.Docs) != len(in.Docs) {
		t.Fatalf("Docs length: got %d, want %d", len(out.Docs), len(in.Docs))
	}
	if out.Docs[0].Body != in.Docs[0].Body || out.Docs[0].LogID != in.Docs[0].LogID {
		t.Errorf("Doc[0]: got %+v, want %+v", out.Docs[0], in.Docs[0])
	}
	if got, want := out.Docs[0].Vec["panic"], in.Docs[0].Vec["panic"]; got != want {
		t.Errorf("Doc[0].Vec[panic]: got %v, want %v", got, want)
	}
	if got, want := out.IDF["panic"], in.IDF["panic"]; got != want {
		t.Errorf("IDF[panic]: got %v, want %v", got, want)
	}
}

// TestDecodeSnapshot_EmptyReader verifies graceful failure on truncation
// at the very first read (magic).
func TestDecodeSnapshot_EmptyReader(t *testing.T) {
	if _, err := DecodeSnapshot(bytes.NewReader(nil)); err == nil {
		t.Fatal("decoding empty reader must fail")
	}
}

// TestDecodeSnapshot_WrongMagic verifies the magic check rejects stray files.
func TestDecodeSnapshot_WrongMagic(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("BAD!")
	_ = binary.Write(&buf, binary.BigEndian, snapshotVersion)
	_ = binary.Write(&buf, binary.BigEndian, uint32(0))
	if _, err := DecodeSnapshot(&buf); err == nil {
		t.Fatal("wrong magic must fail")
	}
}

// TestDecodeSnapshot_WrongVersion verifies version-bump reads are refused
// — the loader should fall back to full rebuild on any version mismatch.
func TestDecodeSnapshot_WrongVersion(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString(snapshotMagic)
	_ = binary.Write(&buf, binary.BigEndian, uint32(999))
	if _, err := DecodeSnapshot(&buf); err == nil {
		t.Fatal("wrong version must fail")
	}
}

// TestDecodeSnapshot_CRCMismatch verifies bit-rot or partial writes are
// caught before the gob decoder produces silently-wrong state.
func TestDecodeSnapshot_CRCMismatch(t *testing.T) {
	in := Snapshot{LastIndexedID: 1, MaxSize: 100, IDF: map[string]float64{}}
	var buf bytes.Buffer
	if err := EncodeSnapshot(&buf, in); err != nil {
		t.Fatalf("encode: %v", err)
	}
	raw := buf.Bytes()
	// Header is 12 bytes (magic+version+crc); flip a payload byte.
	if len(raw) < 13 {
		t.Fatalf("encoded snapshot too short: %d bytes", len(raw))
	}
	raw[12] ^= 0xff
	if _, err := DecodeSnapshot(bytes.NewReader(raw)); err == nil {
		t.Fatal("CRC mismatch must fail")
	}
}

// TestWriteAtomic_RoundTrip writes a payload and reads it back via the
// public path, then asserts the .tmp sibling is gone.
func TestWriteAtomic_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "snap.bin")
	payload := []byte("hello world")
	if err := writeAtomic(p, payload); err != nil {
		t.Fatalf("writeAtomic: %v", err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("round-trip: got %q, want %q", got, payload)
	}
	if _, err := os.Stat(p + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf(".tmp must be removed after rename, got err=%v", err)
	}
}

// TestIsEXDEV_Detection verifies the helper recognizes wrapped EXDEV from
// os.Rename and ignores arbitrary errors.
func TestIsEXDEV_Detection(t *testing.T) {
	le := &os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.EXDEV}
	if !isEXDEV(le) {
		t.Fatal("isEXDEV should detect *os.LinkError{Err: EXDEV}")
	}
	if isEXDEV(errors.New("other error")) {
		t.Fatal("isEXDEV should not flag arbitrary errors")
	}
	if isEXDEV(nil) {
		t.Fatal("isEXDEV(nil) must be false")
	}
}

// TestIndexWriteAndLoadSnapshot exercises the full Index → file → Index
// round trip: build, snapshot, load into a fresh Index, verify state.
func TestIndexWriteAndLoadSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vectordb.snapshot")

	src := New(1000)
	src.Add(101, "acme", "checkout", "ERROR", "payment gateway timeout charging customer")
	src.Add(102, "acme", "checkout", "ERROR", "payment gateway refused charge insufficient funds")
	src.Add(203, "globex", "auth", "WARN", "session token nearing expiry")
	if got, want := src.Size(), 3; got != want {
		t.Fatalf("seed Size: got %d, want %d", got, want)
	}
	if got := src.LastIndexedID(); got != 203 {
		t.Fatalf("LastIndexedID: got %d, want 203", got)
	}

	if err := src.WriteSnapshot(path); err != nil {
		t.Fatalf("WriteSnapshot: %v", err)
	}

	// Verify file written + .tmp gone
	if st, err := os.Stat(path); err != nil {
		t.Fatalf("stat snapshot: %v", err)
	} else if st.Size() == 0 {
		t.Fatal("snapshot file is empty")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf(".tmp must be gone after WriteSnapshot, got err=%v", err)
	}

	dst := New(500) // different cap; load should restore src's cap
	if err := dst.LoadSnapshot(path); err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if got, want := dst.Size(), 3; got != want {
		t.Fatalf("loaded Size: got %d, want %d", got, want)
	}
	if got := dst.LastIndexedID(); got != 203 {
		t.Fatalf("loaded LastIndexedID: got %d, want 203", got)
	}
	// Search should work on the restored index — the IDF table came along
	// with the snapshot, so cosine ranking still has rarity weights.
	hits := dst.Search("acme", "payment gateway", 5)
	if len(hits) != 2 {
		t.Fatalf("Search after load: got %d hits, want 2", len(hits))
	}
}

// TestLoadSnapshot_MissingFile verifies the loader propagates os-level
// errors so callers can distinguish "first start, no snapshot" via
// os.IsNotExist from real corruption.
func TestLoadSnapshot_MissingFile(t *testing.T) {
	dir := t.TempDir()
	idx := New(100)
	err := idx.LoadSnapshot(filepath.Join(dir, "does-not-exist"))
	if err == nil {
		t.Fatal("LoadSnapshot of missing file must error")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("want os.IsNotExist, got %v", err)
	}
}

// TestSnapshotLoop_FinalWriteOnCancel verifies the loop fires a final
// WriteSnapshot when ctx is cancelled — captures the maximum in-memory
// state at graceful shutdown.
func TestSnapshotLoop_FinalWriteOnCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.bin")

	idx := New(100)
	idx.Add(1, "t", "svc", "ERROR", "preserved across shutdown final write")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		// 1h interval — the loop should never tick during this test, only
		// the cancel path fires the write.
		idx.SnapshotLoop(ctx, path, 1*time.Hour)
	}()

	// Sanity: file does not yet exist.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("snapshot must not exist before cancel, got err=%v", err)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SnapshotLoop did not return within 2s of cancel")
	}

	// Verify final write happened.
	if st, err := os.Stat(path); err != nil {
		t.Fatalf("final snapshot missing after cancel: %v", err)
	} else if st.Size() == 0 {
		t.Fatal("final snapshot file is empty")
	}

	// Round-trip: load into a fresh index and confirm state matches.
	dst := New(100)
	if err := dst.LoadSnapshot(path); err != nil {
		t.Fatalf("LoadSnapshot of final write: %v", err)
	}
	if dst.Size() != 1 || dst.LastIndexedID() != 1 {
		t.Fatalf("loaded state mismatch: Size=%d LastIndexedID=%d", dst.Size(), dst.LastIndexedID())
	}
}

// TestSnapshotLoop_PeriodicWrite verifies a tick fires WriteSnapshot.
// Uses a tight interval so the test runs in <50ms; the loop fires at
// least once before we cancel + drain.
func TestSnapshotLoop_PeriodicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.bin")

	idx := New(100)
	idx.Add(7, "t", "svc", "ERROR", "periodic snapshot tick body")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		idx.SnapshotLoop(ctx, path, 10*time.Millisecond)
	}()

	// Wait long enough for at least one tick to fire.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if st, err := os.Stat(path); err == nil && st.Size() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	<-done

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected at least one periodic snapshot to land at %s, got err=%v", path, err)
	}
}

// TestSnapshotLoop_DisabledByEmptyPath verifies the no-op path so config
// disable doesn't accidentally start a tight-loop goroutine.
func TestSnapshotLoop_DisabledByEmptyPath(t *testing.T) {
	idx := New(100)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		idx.SnapshotLoop(ctx, "", 10*time.Millisecond)
	}()
	// Loop should return immediately when path is empty — no need to cancel.
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		cancel()
		t.Fatal("SnapshotLoop with empty path must return immediately")
	}
	cancel()
}

// TestLoadSnapshot_CorruptFileLeavesStateAlone verifies that a corrupt
// snapshot does NOT clobber existing index state — the caller is meant to
// log the warning and proceed with a full rebuild.
func TestLoadSnapshot_CorruptFileLeavesStateAlone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.bin")
	if err := os.WriteFile(path, []byte("not a valid snapshot file"), 0o600); err != nil {
		t.Fatalf("seed corrupt file: %v", err)
	}
	idx := New(100)
	idx.Add(1, "t", "svc", "ERROR", "preexisting body content")
	sizeBefore := idx.Size()
	if err := idx.LoadSnapshot(path); err == nil {
		t.Fatal("LoadSnapshot of corrupt file must fail")
	}
	if got := idx.Size(); got != sizeBefore {
		t.Fatalf("corrupt load corrupted state: Size went %d → %d", sizeBefore, got)
	}
}
