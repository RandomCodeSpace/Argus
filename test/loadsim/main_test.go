//go:build loadtest

package main

import (
	"math"
	"testing"
	"time"
)

// TestServiceName verifies zero-padded naming scheme.
func TestServiceName(t *testing.T) {
	cases := []struct {
		idx  int
		want string
	}{
		{0, "loadsim-svc-000"},
		{1, "loadsim-svc-001"},
		{99, "loadsim-svc-099"},
		{199, "loadsim-svc-199"},
	}
	for _, tc := range cases {
		got := serviceName(tc.idx)
		if got != tc.want {
			t.Errorf("serviceName(%d) = %q, want %q", tc.idx, got, tc.want)
		}
	}
}

// TestSpanFactory verifies round-robin ops, duration range, and ~5% error rate.
func TestSpanFactory(t *testing.T) {
	const samples = 10_000

	opCounts := make(map[string]int)
	errorCount := 0
	tooShort := 0
	tooLong := 0

	for i := 0; i < samples; i++ {
		op := pickOperation(i)
		opCounts[op]++

		dur := randomDuration()
		if dur < 5*time.Millisecond {
			tooShort++
		}
		if dur > 500*time.Millisecond {
			tooLong++
		}

		if isError(i) {
			errorCount++
		}
	}

	// All 5 operations must appear.
	for _, op := range operations {
		if opCounts[op] == 0 {
			t.Errorf("operation %q never selected in %d samples", op, samples)
		}
	}

	// Round-robin: each op should appear exactly samples/5 times.
	expected := samples / len(operations)
	for _, op := range operations {
		if opCounts[op] != expected {
			t.Errorf("operation %q count = %d, want %d (strict round-robin)", op, opCounts[op], expected)
		}
	}

	// Duration must stay in [5ms, 500ms].
	if tooShort > 0 {
		t.Errorf("%d durations < 5ms", tooShort)
	}
	if tooLong > 0 {
		t.Errorf("%d durations > 500ms", tooLong)
	}

	// Error rate: 5% ± 1% (absolute).
	errorRate := float64(errorCount) / float64(samples)
	if math.Abs(errorRate-0.05) > 0.01 {
		t.Errorf("error rate = %.4f, want 0.05 ± 0.01", errorRate)
	}
}

// TestRateLimiter drives the ticker-based limiter for ~1 second and checks throughput.
func TestRateLimiter(t *testing.T) {
	const targetRPS = 50
	rl := newRateLimiter(targetRPS)
	defer rl.stop()

	start := time.Now()
	count := 0
	deadline := start.Add(1 * time.Second)

	for time.Now().Before(deadline) {
		rl.wait()
		count++
	}

	// Allow ±10% of the target (50 ± 5).
	rpsF := float64(targetRPS)
	low := int(rpsF * 0.90)
	high := int(rpsF * 1.10)
	if count < low || count > high {
		t.Errorf("rate limiter issued %d tokens in 1s, want %d–%d (target %d ±10%%)", count, low, high, targetRPS)
	}
}
