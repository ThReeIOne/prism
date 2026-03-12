package sdk

import (
	"context"
	"testing"
)

func TestTracer_StartSpan_RootSpan(t *testing.T) {
	tracer := NewTracer("test-svc", WithSampler(AlwaysSampler{}))
	ctx, span := tracer.StartSpan(context.Background(), "test-op")

	if span.TraceID == "" {
		t.Fatal("expected non-empty TraceID")
	}
	if span.SpanID == "" {
		t.Fatal("expected non-empty SpanID")
	}
	if span.ParentSpanID != "" {
		t.Fatalf("expected empty ParentSpanID for root span, got %q", span.ParentSpanID)
	}
	if span.Operation != "test-op" {
		t.Fatalf("expected operation 'test-op', got %q", span.Operation)
	}
	if span.Service != "test-svc" {
		t.Fatalf("expected service 'test-svc', got %q", span.Service)
	}
	if span.Kind != KindInternal {
		t.Fatalf("expected KindInternal, got %v", span.Kind)
	}
	if span.Status != StatusOK {
		t.Fatalf("expected StatusOK, got %v", span.Status)
	}

	// Should be in context
	fromCtx := SpanFromContext(ctx)
	if fromCtx != span {
		t.Fatal("expected span to be in context")
	}
}

func TestTracer_StartSpan_ChildSpan(t *testing.T) {
	tracer := NewTracer("test-svc")
	ctx, parent := tracer.StartSpan(context.Background(), "parent-op")
	_, child := tracer.StartSpan(ctx, "child-op")

	if child.TraceID != parent.TraceID {
		t.Fatal("child should inherit parent's TraceID")
	}
	if child.ParentSpanID != parent.SpanID {
		t.Fatalf("child ParentSpanID should be parent SpanID: got %q, want %q", child.ParentSpanID, parent.SpanID)
	}
	if child.SpanID == parent.SpanID {
		t.Fatal("child and parent should have different SpanIDs")
	}
}

func TestTracer_StartSpan_WithOptions(t *testing.T) {
	tracer := NewTracer("test-svc")
	_, span := tracer.StartSpan(context.Background(), "op",
		WithKind(KindServer),
		WithTag("key1", "val1"),
	)

	if span.Kind != KindServer {
		t.Fatalf("expected KindServer, got %v", span.Kind)
	}
	if span.Tags["key1"] != "val1" {
		t.Fatalf("expected tag key1=val1, got %q", span.Tags["key1"])
	}
}

func TestTracer_FinishSpan(t *testing.T) {
	tracer := NewTracer("test-svc", WithSampler(NeverSampler{}))
	_, span := tracer.StartSpan(context.Background(), "op")
	time.Sleep(time.Millisecond)
	tracer.FinishSpan(span)

	if span.DurationUs == 0 {
		t.Fatal("expected non-zero DurationUs after FinishSpan")
	}
}

func TestSpanFromContext_Nil(t *testing.T) {
	span := SpanFromContext(context.Background())
	if span != nil {
		t.Fatal("expected nil span from empty context")
	}
}

func TestGenerateID(t *testing.T) {
	id8 := generateID(8)
	if len(id8) != 16 {
		t.Fatalf("expected 16 hex chars for 8 bytes, got %d: %q", len(id8), id8)
	}

	id16 := generateID(16)
	if len(id16) != 32 {
		t.Fatalf("expected 32 hex chars for 16 bytes, got %d: %q", len(id16), id16)
	}

	// Ensure uniqueness
	if id8 == generateID(8) {
		t.Fatal("expected unique IDs")
	}
}
