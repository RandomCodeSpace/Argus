package storage

import (
	"testing"
	"time"
)

func TestPartitionNameForDay_Format(t *testing.T) {
	d := time.Date(2026, 4, 27, 14, 30, 0, 0, time.UTC)
	got := partitionNameForDay(d)
	want := "logs_2026_04_27"
	if got != want {
		t.Fatalf("partitionNameForDay(%s) = %q, want %q", d, got, want)
	}
}

// Two callers in different timezones must converge on the same partition
// name for the same instant — partitionNameForDay normalizes to UTC.
func TestPartitionNameForDay_NormalizesToUTC(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skip("LA tzdata not available")
	}
	utc := time.Date(2026, 4, 27, 6, 0, 0, 0, time.UTC)
	la := utc.In(loc) // same instant, different wall clock
	if partitionNameForDay(utc) != partitionNameForDay(la) {
		t.Fatalf("expected same partition name across TZs: utc=%q la=%q",
			partitionNameForDay(utc), partitionNameForDay(la))
	}
}

func TestQuoteIdent_EscapesEmbeddedQuotes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"logs_2026_04_27", `"logs_2026_04_27"`},
		{`bad"name`, `"bad""name"`},
		{`logs"; DROP TABLE x; --`, `"logs""; DROP TABLE x; --"`},
	}
	for _, c := range cases {
		if got := quoteIdent(c.in); got != c.want {
			t.Fatalf("quoteIdent(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParsePartitionUpper_ValidLayouts(t *testing.T) {
	cases := []struct {
		bound string
		want  string // expected upper, ISO
	}{
		{
			bound: "FOR VALUES FROM ('2026-04-26 00:00:00+00') TO ('2026-04-27 00:00:00+00')",
			want:  "2026-04-27T00:00:00Z",
		},
		{
			bound: "FOR VALUES FROM ('2026-04-26 00:00:00') TO ('2026-04-27 00:00:00')",
			want:  "2026-04-27T00:00:00Z",
		},
	}
	for _, c := range cases {
		t.Run(c.bound, func(t *testing.T) {
			got, ok := parsePartitionUpper(c.bound)
			if !ok {
				t.Fatalf("parsePartitionUpper(%q) failed", c.bound)
			}
			if got.UTC().Format(time.RFC3339) != c.want {
				t.Fatalf("parsePartitionUpper(%q) = %v, want %s", c.bound, got, c.want)
			}
		})
	}
}

func TestParsePartitionUpper_Malformed(t *testing.T) {
	cases := []string{
		"",
		"FOR VALUES FROM ('2026-04-26 00:00:00+00')", // no TO
		"FOR VALUES FROM ('xxx') TO ('not a timestamp')",
		"random garbage",
	}
	for _, c := range cases {
		if _, ok := parsePartitionUpper(c); ok {
			t.Fatalf("parsePartitionUpper(%q) should have failed", c)
		}
	}
}
