package collector

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/shengli/prism/internal/metrics"
	"github.com/shengli/prism/internal/storage"
	prismpb "github.com/shengli/prism/proto/gen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Collector receives spans from SDKs and writes them to storage in batches.
type Collector struct {
	prismpb.UnimplementedCollectorServiceServer
	buffer        []*prismpb.Span
	recordBuffer  []storage.SpanRecord // for HTTP ingest (already converted)
	mu            sync.Mutex
	flushSize     int
	maxBuffer     int
	flushInterval time.Duration
	store         storage.Storage
	depTracker    *DependencyTracker
	done          chan struct{}
}

// Config holds collector configuration.
type Config struct {
	FlushSize     int
	FlushInterval time.Duration
	MaxBuffer     int
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
	if cfg.MaxBuffer <= 0 {
		cfg.MaxBuffer = 100000
	}
	c := &Collector{
		buffer:        make([]*prismpb.Span, 0, cfg.FlushSize),
		flushSize:     cfg.FlushSize,
		maxBuffer:     cfg.MaxBuffer,
		flushInterval: cfg.FlushInterval,
		store:         cfg.Store,
		depTracker:    cfg.DepTracker,
		done:          make(chan struct{}),
	}
	// Publish the configured buffer capacity for use-rate calculation.
	metrics.BufferCapacity.Set(float64(cfg.MaxBuffer))
	go c.flushLoop()
	return c
}

// Report implements the gRPC CollectorService.Report method.
func (c *Collector) Report(ctx context.Context, batch *prismpb.SpanBatch) (*prismpb.ReportResponse, error) {
	spanCount := len(batch.Spans)

	c.mu.Lock()
	if len(c.buffer)+len(c.recordBuffer) >= c.maxBuffer {
		c.mu.Unlock()
		metrics.SpansDropped.Add(float64(spanCount))
		return nil, status.Errorf(codes.ResourceExhausted, "collector buffer full")
	}
	metrics.SpansReceived.Add(float64(spanCount))
	metrics.SDKReportBatchSize.Observe(float64(spanCount))
	for _, span := range batch.Spans {
		if c.depTracker != nil {
			c.depTracker.Record(span)
		}
		c.buffer = append(c.buffer, span)
	}
	shouldFlush := len(c.buffer)+len(c.recordBuffer) >= c.flushSize
	metrics.BufferSize.Set(float64(len(c.buffer) + len(c.recordBuffer)))
	c.mu.Unlock()

	if shouldFlush {
		go c.flush()
	}

	return &prismpb.ReportResponse{Accepted: int32(spanCount)}, nil
}

func (c *Collector) flush() {
	c.mu.Lock()
	if len(c.buffer) == 0 && len(c.recordBuffer) == 0 {
		c.mu.Unlock()
		return
	}
	protoBatch := c.buffer
	httpBatch := c.recordBuffer
	c.buffer = make([]*prismpb.Span, 0, c.flushSize)
	c.recordBuffer = nil
	metrics.BufferSize.Set(0)
	c.mu.Unlock()

	records := make([]storage.SpanRecord, 0, len(protoBatch)+len(httpBatch))
	for _, sp := range protoBatch {
		records = append(records, protoToRecord(sp))
	}
	records = append(records, httpBatch...)

	metrics.FlushBatchSize.Observe(float64(len(records)))

	start := time.Now()

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 500 * time.Millisecond
			select {
			case <-c.done:
				slog.Warn("flush retry aborted due to shutdown")
				return
			case <-time.After(backoff):
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		lastErr = c.store.BatchInsert(ctx, records)
		cancel()
		if lastErr == nil {
			slog.Info("flushed spans to storage", "count", len(records))
			break
		}
		slog.Warn("flush attempt failed", "attempt", attempt+1, "error", lastErr)
	}
	if lastErr != nil {
		slog.Error("flush to storage failed after retries", "error", lastErr, "count", len(records))
		metrics.FlushErrors.Inc()
		metrics.SpansDropped.Add(float64(len(records)))
	}
	metrics.FlushDuration.Observe(time.Since(start).Seconds())
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

// bufferRecords adds pre-converted SpanRecords to the buffer (used by HTTP ingest).
// Returns true if the buffer is full and the records were dropped.
func (c *Collector) bufferRecords(records []storage.SpanRecord) bool {
	c.mu.Lock()
	if len(c.buffer)+len(c.recordBuffer) >= c.maxBuffer {
		c.mu.Unlock()
		metrics.SpansDropped.Add(float64(len(records)))
		return true
	}
	c.recordBuffer = append(c.recordBuffer, records...)
	shouldFlush := len(c.buffer) >= c.flushSize || len(c.recordBuffer) >= c.flushSize
	metrics.BufferSize.Set(float64(len(c.buffer) + len(c.recordBuffer)))
	c.mu.Unlock()

	if shouldFlush {
		go c.flush()
	}
	return false
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
