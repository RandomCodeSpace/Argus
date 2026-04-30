package vectordb

import "context"

// ReplaySource is the minimal contract a backing store fulfills to hydrate
// this Index on startup. Pages are pulled in id-ascending order; the source
// signals end-of-data by returning a slice shorter than the requested limit.
// ReplayFromDB walks pages starting from LastIndexedID() until the source
// returns no more rows.
//
// Vectordb intentionally does NOT import the storage package — keeping it as
// a leaf accelerator means tests can wire any in-memory source without a
// SQLite dependency, and storage is free to evolve its row type without
// breaking vectordb. The wiring layer (cmd/main.go) is responsible for
// projecting storage.Log into ReplayRow.
type ReplaySource interface {
	LogsForVectorReplay(ctx context.Context, sinceID uint, limit int) ([]ReplayRow, error)
}

// ReplayRow is the minimum field set Add() needs. Mirrors the projection a
// storage adapter performs at the boundary.
type ReplayRow struct {
	ID          uint
	Tenant      string
	ServiceName string
	Severity    string
	Body        string
}

// replayPageSize bounds memory during tail-replay. 10k rows is a reasonable
// trade-off between query overhead per page and peak heap; at typical body
// sizes this stays well under 50 MB resident per page.
const replayPageSize = 10_000

// ReplayFromDB walks ReplaySource pages starting from LastIndexedID() and
// feeds each row through Add(). Returns the count of rows processed (Add
// filters by severity, so processed ≠ indexed when the source loosens its
// filter — but the standard storage implementation already pre-filters to
// ERROR/WARN/family so the counts match in practice).
//
// Termination contract: the source signals end-of-data by returning a
// zero-length slice. This lets sources page however they want without
// having to fill every page exactly to replayPageSize — the trade-off is
// one extra round-trip at the tail (fine for a one-shot startup call).
//
// Caller passes a derived ctx so SIGTERM during boot cancels the replay
// cleanly. On any source error, returns the partial count + error so the
// caller can log and proceed with a partially-warm index.
func (idx *Index) ReplayFromDB(ctx context.Context, src ReplaySource) (int, error) {
	if src == nil {
		return 0, nil
	}
	sinceID := idx.LastIndexedID()
	total := 0
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		rows, err := src.LogsForVectorReplay(ctx, sinceID, replayPageSize)
		if err != nil {
			return total, err
		}
		if len(rows) == 0 {
			return total, nil
		}
		for _, row := range rows {
			idx.Add(row.ID, row.Tenant, row.ServiceName, row.Severity, row.Body)
			if row.ID > sinceID {
				sinceID = row.ID
			}
		}
		total += len(rows)
	}
}
