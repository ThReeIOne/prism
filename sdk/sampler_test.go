package sdk

import (
	"testing"
)

func TestAlwaysSampler(t *testing.T) {
	s := AlwaysSampler{}
	span := &Span{TraceID: "test123"}
	if !s.ShouldSample(span) {
		t.Fatal("AlwaysSampler should always return true")
	}
}

func TestNeverSampler(t *testing.T) {
	s := NeverSampler{}
	span := &Span{TraceID: "test123"}
	if s.ShouldSample(span) {
		t.Fatal("NeverSampler should always return false")
	}
}

func TestAdaptiveSampler_ErrorAlwaysSampled(t *testing.T) {
	s := NewAdaptiveSampler(0.0, 1000) // 0% base rate
	span := &Span{
		TraceID: "test123",
		Status:  StatusError,
	}
	if !s.ShouldSample(span) {
		t.Fatal("errors should always be sampled")
	}
}

func TestAdaptiveSampler_SlowRequestAlwaysSampled(t *testing.T) {
	s := NewAdaptiveSampler(0.0, 1000) // 0% base rate
	span := &Span{
		TraceID:    "test123",
		DurationUs: 2_000_000, // 2 seconds
		Status:     StatusOK,
	}
	if !s.ShouldSample(span) {
		t.Fatal("slow requests (>1s) should always be sampled")
	}
}

func TestAdaptiveSampler_RateLimit(t *testing.T) {
	s := NewAdaptiveSampler(1.0, 2) // 100% rate but max 2/s
	span := &Span{
		TraceID: "test123",
		Status:  StatusOK,
	}

	sampled := 0
	for i := 0; i < 100; i++ {
		if s.ShouldSample(span) {
			sampled++
		}
	}
	// Should be capped at 2 (the maxPerSec)
	if sampled > 3 { // Allow a little slack for the counter reset race
		t.Fatalf("expected at most ~2 sampled, got %d", sampled)
	}
}

func TestAdaptiveSampler_ConsistentPerTrace(t *testing.T) {
	s := NewAdaptiveSampler(0.5, 100000)
	span := &Span{
		TraceID: "consistent-trace-id-12345",
		Status:  StatusOK,
	}

	first := s.ShouldSample(span)
	// Reset counter so rate limit doesn't interfere
	s.counter.Store(0)
	second := s.ShouldSample(span)

	if first != second {
		t.Fatal("same trace_id should produce consistent sampling decision")
	}
}

func TestFnv32(t *testing.T) {
	h1 := fnv32("hello")
	h2 := fnv32("world")
	if h1 == h2 {
		t.Fatal("different strings should produce different hashes")
	}

	// Deterministic
	if fnv32("test") != fnv32("test") {
		t.Fatal("same string should produce same hash")
	}
}
