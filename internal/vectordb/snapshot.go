package vectordb

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"log/slog"
	"os"
	"syscall"
	"time"
)

// Snapshot is the persisted state of an Index.
//
// Only the fields needed to reconstruct an equivalent Index are captured —
// transient state (mu, dirty) is intentionally absent. LastIndexedID is the
// high watermark of indexed Log.IDs so a startup tail-replay can query DB
// rows newer than the snapshot without double-indexing rows already in
// Docs.
//
// Field changes break the format — bump snapshotVersion when the wire
// shape changes. Old snapshots whose magic+version don't match are
// rejected on load and the caller falls back to a full DB rebuild.
type Snapshot struct {
	LastIndexedID uint
	MaxSize       int
	Docs          []LogVector
	IDF           map[string]float64
	WrittenAt     int64 // unix seconds, observability only
}

const (
	// snapshotMagic is a 4-byte file header so a corrupt or stray file is
	// rejected before we attempt the more expensive gob decode.
	snapshotMagic = "VDB1"
	// snapshotVersion travels alongside the magic. Bump on any LogVector
	// or Snapshot field shape change so loaders fall back to rebuild
	// instead of producing silently-wrong index state.
	snapshotVersion uint32 = 1
)

// EncodeSnapshot writes a versioned, CRC32-protected snapshot to w.
//
// Wire format (big-endian for portability):
//
//	bytes[0:4]   magic       "VDB1"
//	bytes[4:8]   version     uint32
//	bytes[8:12]  CRC32-IEEE  uint32 (over bytes[12:])
//	bytes[12:]   gob payload Snapshot
func EncodeSnapshot(w io.Writer, snap Snapshot) error {
	var payload bytes.Buffer
	if err := gob.NewEncoder(&payload).Encode(snap); err != nil {
		return fmt.Errorf("encode snapshot payload: %w", err)
	}
	crc := crc32.ChecksumIEEE(payload.Bytes())

	if _, err := w.Write([]byte(snapshotMagic)); err != nil {
		return fmt.Errorf("write magic: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, snapshotVersion); err != nil {
		return fmt.Errorf("write version: %w", err)
	}
	if err := binary.Write(w, binary.BigEndian, crc); err != nil {
		return fmt.Errorf("write crc: %w", err)
	}
	if _, err := w.Write(payload.Bytes()); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	return nil
}

// DecodeSnapshot reads + validates a snapshot from r.
//
// All errors are caller-visible. The expected handling is: log a warning
// and proceed with a full DB rebuild — never silently load partial state.
// Errors include short header, wrong magic, unsupported version, CRC
// mismatch, and gob decode failure.
func DecodeSnapshot(r io.Reader) (Snapshot, error) {
	var (
		magic   [4]byte
		version uint32
		crc     uint32
	)
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return Snapshot{}, fmt.Errorf("read magic: %w", err)
	}
	if string(magic[:]) != snapshotMagic {
		return Snapshot{}, fmt.Errorf("unexpected snapshot magic %q (want %q)", magic[:], snapshotMagic)
	}
	if err := binary.Read(r, binary.BigEndian, &version); err != nil {
		return Snapshot{}, fmt.Errorf("read version: %w", err)
	}
	if version != snapshotVersion {
		return Snapshot{}, fmt.Errorf("unsupported snapshot version %d (current %d)", version, snapshotVersion)
	}
	if err := binary.Read(r, binary.BigEndian, &crc); err != nil {
		return Snapshot{}, fmt.Errorf("read crc: %w", err)
	}
	payload, err := io.ReadAll(r)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read payload: %w", err)
	}
	if got := crc32.ChecksumIEEE(payload); got != crc {
		return Snapshot{}, fmt.Errorf("snapshot crc mismatch: got %08x want %08x", got, crc)
	}
	var snap Snapshot
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&snap); err != nil {
		return Snapshot{}, fmt.Errorf("decode payload: %w", err)
	}
	return snap, nil
}

// writeAtomic writes data to path via tmp+sync+rename.
//
// Mode 0o600: snapshots persist log bodies which can carry sensitive
// operational data — owner-only is the conservative default. Operators
// who need shared read can chmod externally.
//
// On EXDEV (cross-device rename, e.g. when data dir is on a separate
// mount than the binary's tmp dir), falls back to a non-atomic
// os.WriteFile at the destination. Cross-device deployments are rare and
// documented; the fallback at least ensures the snapshot is written, with
// last-writer-wins replacing the atomicity guarantee.
//
// On any error during the write/fsync phase, the .tmp file is removed so
// a partial file does not poison the next startup's load attempt.
func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create tmp %s: %w", tmp, err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("fsync tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		if isEXDEV(err) {
			data, readErr := os.ReadFile(tmp)
			if readErr != nil {
				_ = os.Remove(tmp)
				return fmt.Errorf("rename EXDEV + readback: %w", readErr)
			}
			if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
				_ = os.Remove(tmp)
				return fmt.Errorf("rename EXDEV + writefile: %w", writeErr)
			}
			_ = os.Remove(tmp)
			return nil
		}
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}

// isEXDEV reports whether err is a cross-device link/rename error.
func isEXDEV(err error) bool {
	if err == nil {
		return false
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return errors.Is(le.Err, syscall.EXDEV)
	}
	return errors.Is(err, syscall.EXDEV)
}

// LoadSnapshot reads a snapshot from path and replaces the Index's state.
//
// Caller must ensure no concurrent Add()/Search() is in flight — this is
// the typical startup wiring (fresh Index, before ingest accept). Errors
// are returned as-is so the caller can distinguish os.IsNotExist (no
// previous snapshot — first start) from corruption/format errors (log
// warn + proceed with full DB rebuild).
//
// On error the Index state is left untouched.
func (idx *Index) LoadSnapshot(path string) error {
	f, err := os.Open(path) // #nosec G304 -- operator-supplied snapshot path
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	snap, err := DecodeSnapshot(f)
	if err != nil {
		return err
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.docs = snap.Docs
	idx.idf = snap.IDF
	if idx.idf == nil {
		idx.idf = make(map[string]float64)
	}
	if snap.MaxSize > 0 {
		idx.maxSize = snap.MaxSize
	}
	idx.lastIndexedID = snap.LastIndexedID
	idx.dirty = false
	return nil
}

// SetSnapshotObserver registers a callback invoked at the end of each
// WriteSnapshot. result is "success" or "failure"; size is the on-disk
// size of the latest written snapshot (0 on failure).
//
// Set from the wiring layer (main.go) so vectordb stays free of
// telemetry imports. Safe to call before SnapshotLoop starts.
func (idx *Index) SetSnapshotObserver(fn func(result string, duration time.Duration, size int64)) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.snapshotObserver = fn
}

// WriteSnapshot serializes the current Index state to path atomically.
//
// Safe to call concurrently with Add()/Search(): the docs slice and IDF
// map are copied under the index lock and serialization runs lock-free
// after release. Critical section is sub-millisecond at the 100k cap
// because slice copy is O(1) per-element header (LogVector strings/maps
// are shared by reference, and Add() never mutates an existing
// LogVector.Vec — it only appends new entries).
func (idx *Index) WriteSnapshot(path string) error {
	start := time.Now()
	err := idx.writeSnapshot(path)

	idx.mu.RLock()
	obs := idx.snapshotObserver
	idx.mu.RUnlock()
	if obs != nil {
		result := "success"
		var size int64
		if err != nil {
			result = "failure"
		} else if fi, statErr := os.Stat(path); statErr == nil {
			size = fi.Size()
		}
		obs(result, time.Since(start), size)
	}
	return err
}

func (idx *Index) writeSnapshot(path string) error {
	idx.mu.Lock()
	if idx.dirty {
		idx.recomputeIDF()
		idx.dirty = false
	}
	docs := make([]LogVector, len(idx.docs))
	copy(docs, idx.docs)
	idfCopy := make(map[string]float64, len(idx.idf))
	for k, v := range idx.idf {
		idfCopy[k] = v
	}
	snap := Snapshot{
		LastIndexedID: idx.lastIndexedID,
		MaxSize:       idx.maxSize,
		Docs:          docs,
		IDF:           idfCopy,
		WrittenAt:     time.Now().Unix(),
	}
	idx.mu.Unlock()

	var buf bytes.Buffer
	if err := EncodeSnapshot(&buf, snap); err != nil {
		return err
	}
	return writeAtomic(path, buf.Bytes())
}

// SnapshotLoop writes a snapshot to path on every interval tick until ctx is
// done. On context cancel, fires one final WriteSnapshot before returning so
// graceful shutdowns capture the maximum in-memory state.
//
// Transient write failures (disk full, fsync errors, EXDEV warnings) are
// logged via slog but do not break the loop — vectordb is a rebuildable
// accelerator, and silently dropping a tick beats taking the daemon down.
//
// Safe to call with empty path / zero interval — both disable the loop and
// return immediately.
func (idx *Index) SnapshotLoop(ctx context.Context, path string, interval time.Duration) {
	if path == "" || interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if err := idx.WriteSnapshot(path); err != nil {
				slog.Warn("vectordb final snapshot on shutdown failed", "path", path, "error", err)
			} else {
				slog.Info("vectordb final snapshot written", "path", path, "size", idx.Size())
			}
			return
		case <-ticker.C:
			if err := idx.WriteSnapshot(path); err != nil {
				slog.Warn("vectordb periodic snapshot failed", "path", path, "error", err)
			}
		}
	}
}
