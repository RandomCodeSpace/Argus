package storage

import (
	"errors"
	"time"
)

// ClampSearchWindowTo24h enforces a 24-hour ceiling on log keyword search
// windows applied at the API/MCP boundary. With FTS5 disabled (the default),
// the only safety mechanism for unscoped LIKE substring scans is a hard
// time-range cap so the worst-case query is bounded in disk pages scanned.
//
// Behaviour:
//
//   - Both zero (no caller hint): defaults to (now-24h, now)
//   - end zero: end = now
//   - start zero: start = end - 24h
//   - end > now: end clamped to now (don't reject — clock skew on the caller
//     side is common)
//   - start < now-24h with end >= now-24h: start clamped to now-24h (the
//     window is shrunk to fit the cap, callers never see a silent truncation
//     of an in-window query)
//   - end < now-24h (window entirely outside the cap): rejected with error so
//     the caller gets a deterministic signal instead of an empty result
//   - start >= end: rejected with error (caller mistake)
//
// The cap fires whenever a body keyword search is requested; pure filtered
// listings (no Search field) keep the full retention range and bypass this
// helper.
func ClampSearchWindowTo24h(start, end, now time.Time) (time.Time, time.Time, error) {
	cutoff := now.Add(-24 * time.Hour)
	if end.IsZero() {
		end = now
	}
	if start.IsZero() {
		start = end.Add(-24 * time.Hour)
	}
	if end.After(now) {
		end = now
	}
	if end.Before(cutoff) {
		return time.Time{}, time.Time{}, errors.New("search window must be within the last 24h")
	}
	if start.Before(cutoff) {
		start = cutoff
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, errors.New("start_time must be before end_time")
	}
	return start, end, nil
}
