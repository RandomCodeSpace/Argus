package storage

import (
	"testing"
	"time"
)

func TestClampSearchWindowTo24h(t *testing.T) {
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	hourAgo := now.Add(-1 * time.Hour)
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)
	hourAhead := now.Add(1 * time.Hour)

	cases := []struct {
		name      string
		start     time.Time
		end       time.Time
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			name:      "both zero defaults to last 24h",
			wantStart: dayAgo,
			wantEnd:   now,
		},
		{
			// With end=1h ago and start unset, start is initialized to
			// end-24h (= 25h ago) and then clamped to the cutoff (24h ago)
			// so the resulting window cannot extend past the 24h floor.
			// Window shrinks to 23h — caller sees a strict ceiling on age.
			name:      "only end set: start clamps to cutoff (not end-24h)",
			end:       hourAgo,
			wantStart: dayAgo,
			wantEnd:   hourAgo,
		},
		{
			name:      "start oversize is clamped to cutoff",
			start:     weekAgo,
			end:       now,
			wantStart: dayAgo,
			wantEnd:   now,
		},
		{
			name:      "end in the future is clamped to now",
			start:     hourAgo,
			end:       hourAhead,
			wantStart: hourAgo,
			wantEnd:   now,
		},
		{
			name:    "window entirely outside cap is rejected",
			start:   now.Add(-5 * 24 * time.Hour),
			end:     now.Add(-4 * 24 * time.Hour),
			wantErr: true,
		},
		{
			name:      "narrow in-cap window is unchanged",
			start:     hourAgo,
			end:       now.Add(-30 * time.Minute),
			wantStart: hourAgo,
			wantEnd:   now.Add(-30 * time.Minute),
		},
		{
			name:    "start equal to end is rejected",
			start:   hourAgo,
			end:     hourAgo,
			wantErr: true,
		},
		{
			name:    "start after end is rejected",
			start:   now.Add(-30 * time.Minute),
			end:     hourAgo,
			wantErr: true,
		},
		{
			name:      "start at cutoff boundary is allowed",
			start:     dayAgo,
			end:       now,
			wantStart: dayAgo,
			wantEnd:   now,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotStart, gotEnd, err := ClampSearchWindowTo24h(c.start, c.end, now)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got start=%v end=%v", gotStart, gotEnd)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !gotStart.Equal(c.wantStart) {
				t.Errorf("start: got %v, want %v", gotStart, c.wantStart)
			}
			if !gotEnd.Equal(c.wantEnd) {
				t.Errorf("end: got %v, want %v", gotEnd, c.wantEnd)
			}
		})
	}
}
