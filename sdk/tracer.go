package sdk

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

type spanContextKey struct{}

// Tracer is the main entry point of the Prism SDK.
type Tracer struct {
	service  string
	exporter *BatchExporter
	sampler  Sampler
}

// NewTracer creates a new Tracer instance.
func NewTracer(service string, opts ...Option) *Tracer {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}
	t := &Tracer{
		service:  service,
		exporter: NewBatchExporter(cfg.CollectorAddr, cfg.BatchSize, cfg.FlushInterval),
		sampler:  cfg.Sampler,
	}
	return t
}

// StartSpan creates a new span and injects it into the context.
func (t *Tracer) StartSpan(ctx context.Context, operation string, opts ...SpanOption) (context.Context, *Span) {
	parent := SpanFromContext(ctx)

	span := &Span{
		TraceID:   t.resolveTraceID(parent),
		SpanID:    generateID(8),
		Operation: operation,
		Service:   t.service,
		Kind:      KindInternal,
		StartUs:   uint64(time.Now().UnixMicro()),
		Status:    StatusOK,
		Tags:      make(map[string]string),
	}

	if parent != nil {
		span.ParentSpanID = parent.SpanID
	}

	for _, o := range opts {
		o(span)
	}

	return context.WithValue(ctx, spanContextKey{}, span), span
}

// FinishSpan ends a span and enqueues it for export.
func (t *Tracer) FinishSpan(span *Span) {
	span.DurationUs = uint64(time.Now().UnixMicro()) - span.StartUs
	if t.sampler.ShouldSample(span) {
		t.exporter.Enqueue(span)
	}
}

// Shutdown flushes remaining spans and closes the exporter.
func (t *Tracer) Shutdown() {
	t.exporter.Shutdown()
}

func (t *Tracer) resolveTraceID(parent *Span) string {
	if parent != nil {
		return parent.TraceID
	}
	return generateID(16)
}

// SpanFromContext retrieves the current span from the context.
func SpanFromContext(ctx context.Context) *Span {
	if s, ok := ctx.Value(spanContextKey{}).(*Span); ok {
		return s
	}
	return nil
}

func generateID(byteLen int) string {
	b := make([]byte, byteLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
