package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	prismpb "github.com/shengli/prism/proto/gen"
)

// DependencyTracker extracts service-to-service dependencies from spans
// and caches them in Redis for fast lookup.
type DependencyTracker struct {
	redis *redis.Client
}

// NewDependencyTracker creates a new dependency tracker.
func NewDependencyTracker(redisClient *redis.Client) *DependencyTracker {
	return &DependencyTracker{redis: redisClient}
}

// Record inspects a span and records any cross-service dependency.
func (d *DependencyTracker) Record(span *prismpb.Span) {
	if d.redis == nil {
		return
	}
	if span.Kind != prismpb.SpanKind_CLIENT {
		return
	}
	if len(span.ParentSpanId) == 0 {
		return
	}

	peerService := ""
	for _, tag := range span.Tags {
		if tag.Key == "peer.service" {
			peerService = tag.Value
			break
		}
	}
	if peerService == "" {
		return
	}

	key := fmt.Sprintf("prism:dep:%s:%s", span.Service, peerService)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	d.redis.Incr(ctx, key)
	d.redis.Expire(ctx, key, 24*time.Hour)
}
