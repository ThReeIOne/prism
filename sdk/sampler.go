package sdk

import (
	"sync/atomic"
	"time"
)

// Sampler decides whether a span should be exported.
type Sampler interface {
	ShouldSample(span *Span) bool
}

// AlwaysSampler samples every span.
type AlwaysSampler struct{}

func (AlwaysSampler) ShouldSample(*Span) bool { return true }

// NeverSampler drops every span.
type NeverSampler struct{}

func (NeverSampler) ShouldSample(*Span) bool { return false }

// AdaptiveSampler implements an adaptive sampling strategy.
type AdaptiveSampler struct {
	baseRate  float64
	maxPerSec int64
	counter   atomic.Int64
	lastReset atomic.Int64
}

// NewAdaptiveSampler creates a new adaptive sampler.
// baseRate is a probability from 0.0 to 1.0.
// maxPerSec is the maximum number of sampled spans per second.
func NewAdaptiveSampler(baseRate float64, maxPerSec int64) *AdaptiveSampler {
	s := &AdaptiveSampler{
		baseRate:  baseRate,
		maxPerSec: maxPerSec,
	}
	s.lastReset.Store(time.Now().Unix())
	return s
}

func (s *AdaptiveSampler) ShouldSample(span *Span) bool {
	// Rule 1: always sample errors
	if span.Status == StatusError {
		return true
	}

	// Rule 2: always sample slow requests (>1s)
	if span.DurationUs > 1_000_000 {
		return true
	}

	// Rule 3: rate limiting (CAS to avoid reset race)
	now := time.Now().Unix()
	if last := s.lastReset.Load(); now != last {
		if s.lastReset.CompareAndSwap(last, now) {
			s.counter.Store(0)
		}
	}
	if s.counter.Load() >= s.maxPerSec {
		return false
	}

	// Rule 4: hash-based probabilistic sampling (consistent per trace)
	h := fnv32(span.TraceID)
	sampled := float64(h%10000) < s.baseRate*10000
	if sampled {
		s.counter.Add(1)
	}
	return sampled
}

func fnv32(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h *= 16777619
		h ^= uint32(s[i])
	}
	return h
}
