package collector

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/shengli/prism/internal/storage"
	prismpb "github.com/shengli/prism/proto/gen"
)

// Collector receives spans from SDKs and writes them to storage in batches.
type Collector struct {
	prismpb.UnimplementedCollectorServiceServer
	buffer        []*prismpb.Span
	mu            sync.Mutex
	flushSize     int
	flushInterval time.Duration
	store         storage.Storage
	depTracker    *DependencyTracker
	done          chan struct{}
}

// Config holds collector configuration.
type Config struct {
	FlushSize     int
	FlushInterval time.Duration
	Store         storage.Storage
	DepTracker    *DependencyTracker
}

// New creates a new Collector.
func New(cfg Config) *Collector {
	if cfg.FlushSize <= 0 {
		cfg.FlushSize = 5000
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	c := &Collector{
		buffer:        make([]*prismpb.Span, 0, cfg.FlushSize),
		flushSize:     cfg.FlushSize,
		flushInterval: cfg.FlushInterval,
		store:         cfg.Store,
		depTracker:    cfg.DepTracker,
		done:          make(chan struct{}),
	}
	go c.flushLoop()
	return c
}

// Report implements the gRPC CollectorService.Report method.
func (c *Collector) Report(ctx context.Context, batch *prismpb.SpanBatch) (*prismpb.ReportResponse, error) {
	c.mu.Lock()
	for _, span := range batch.Spans {
		if c.depTracker != nil {
			c.depTracker.Record(span)
		}
		c.buffer = append(c.buffer, span)
	}
	shouldFlush := len(c.buffer) >= c.flushSize
	c.mu.Unlock()

	if shouldFlush {
		go c.flush()
	}

	return &prismpb.ReportResponse{Accepted: int32(len(batch.Spans))}, nil
}

func (c *Collector) flush() {
	c.mu.Lock()
	if len(c.buffer) == 0 {
		c.mu.Unlock()
		return
	}
	batch := c.buffer
	c.buffer = make([]*prismpb.Span, 0, c.flushSize)
	c.mu.Unlock()

	records := make([]storage.SpanRecord, 0, len(batch))
	for _, sp := range batch {
		records = append(records, protoToRecord(sp))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := c.store.BatchInsert(ctx, records); err != nil {
		slog.Error("flush to storage failed", "error", err, "count", len(records))
	} else {
		slog.Info("flushed spans to storage", "count", len(records))
	}
}

func (c *Collector) flushLoop() {
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.flush()
		case <-c.done:
			return
		}
	}
}

// Shutdown flushes remaining spans and stops the collector.
func (c *Collector) Shutdown() {
	close(c.done)
	c.flush()
}

func protoToRecord(sp *prismpb.Span) storage.SpanRecord {
	tags := make(map[string]string)
	for _, kv := range sp.Tags {
		tags[kv.Key] = kv.Value
	}

	var eventsJSON string
	if len(sp.Events) > 0 {
		type eventJSON struct {
			TimestampUs uint64            `json:"timestamp_us"`
			Name        string            `json:"name"`
			Attributes  map[string]string `json:"attributes,omitempty"`
		}
		events := make([]eventJSON, 0, len(sp.Events))
		for _, e := range sp.Events {
			attrs := make(map[string]string)
			for _, kv := range e.Attributes {
				attrs[kv.Key] = kv.Value
			}
			events = append(events, eventJSON{
				TimestampUs: e.TimestampUs,
				Name:        e.Name,
				Attributes:  attrs,
			})
		}
		b, _ := json.Marshal(events)
		eventsJSON = string(b)
	}

	return storage.SpanRecord{
		TraceID:      hex.EncodeToString(sp.TraceId),
		SpanID:       hex.EncodeToString(sp.SpanId),
		ParentSpanID: hex.EncodeToString(sp.ParentSpanId),
		Operation:    sp.Operation,
		Service:      sp.Service,
		Kind:         prismpb.SpanKind_name[int32(sp.Kind)],
		StartUs:      sp.StartUs,
		DurationUs:   sp.DurationUs,
		Status:       statusCodeToString(sp.Status),
		Tags:         tags,
		Events:       eventsJSON,
	}
}

func statusCodeToString(code prismpb.StatusCode) string {
	if code == prismpb.StatusCode_ERROR {
		return "error"
	}
	return "ok"
}
