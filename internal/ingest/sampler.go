package ingest

import (
	"sync"
	"sync/atomic"
	"time"
)

// Sampler decides whether a trace/span should be ingested.
// Always keeps: error traces, slow traces (duration > latencyThresholdMs), new services.
// Samples healthy traces at the configured rate using a per-service token bucket.
type Sampler struct {
	rate                float64 // 0.0–1.0, fraction to keep
	alwaysOnErrors      bool
	latencyThresholdMs  float64 // always keep traces slower than this
	mu                  sync.Mutex
	buckets             map[string]*tokenBucket
	totalSeen           atomic.Int64
	totalDropped        atomic.Int64
}

// NewSampler creates a Sampler with the given parameters.
func NewSampler(rate float64, alwaysOnErrors bool, latencyThresholdMs float64) *Sampler {
	if rate <= 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	return &Sampler{
		rate:               rate,
		alwaysOnErrors:     alwaysOnErrors,
		latencyThresholdMs: latencyThresholdMs,
		buckets:            make(map[string]*tokenBucket),
	}
}

// ShouldSample returns true if the trace should be ingested.
// isError: whether the trace/span has error status.
// durationMs: trace duration in milliseconds.
// serviceName: originating service.
func (s *Sampler) ShouldSample(serviceName string, isError bool, durationMs float64) bool {
	s.totalSeen.Add(1)

	// Always ingest errors.
	if s.alwaysOnErrors && isError {
		return true
	}

	// Always ingest slow traces.
	if durationMs >= s.latencyThresholdMs {
		return true
	}

	// Full ingestion rate — skip sampling.
	if s.rate >= 1.0 {
		return true
	}

	// Zero rate — drop everything (except errors/slow, handled above).
	if s.rate <= 0 {
		s.totalDropped.Add(1)
		return false
	}

	// Token bucket per service.
	s.mu.Lock()
	b, ok := s.buckets[serviceName]
	if !ok {
		b = newTokenBucket(s.rate)
		s.buckets[serviceName] = b
		// Always let first trace through (new service discovery).
		s.mu.Unlock()
		return true
	}
	allow := b.allow()
	s.mu.Unlock()

	if !allow {
		s.totalDropped.Add(1)
	}
	return allow
}

// Stats returns (seen, dropped) counters for metrics.
func (s *Sampler) Stats() (int64, int64) {
	return s.totalSeen.Load(), s.totalDropped.Load()
}

// tokenBucket is a simple token bucket for sampling decisions.
// Refills at `rate` tokens per second, max capacity 1.0.
type tokenBucket struct {
	rate     float64   // tokens per second
	tokens   float64   // current tokens (0.0–1.0)
	lastTick time.Time
}

func newTokenBucket(rate float64) *tokenBucket {
	return &tokenBucket{
		rate:     rate,
		tokens:   rate, // start with one full refill worth
		lastTick: time.Now(),
	}
}

// allow returns true and consumes a token if one is available.
func (b *tokenBucket) allow() bool {
	now := time.Now()
	elapsed := now.Sub(b.lastTick).Seconds()
	b.lastTick = now

	b.tokens += elapsed * b.rate
	if b.tokens > 1.0 {
		b.tokens = 1.0
	}

	if b.tokens >= 1.0/b.rate {
		b.tokens -= 1.0 / b.rate
		return true
	}
	return false
}
