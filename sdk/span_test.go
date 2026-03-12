package sdk

import (
	"errors"
	"testing"
)

func TestSpan_SetTag(t *testing.T) {
	s := &Span{Tags: make(map[string]string)}
	s.SetTag("key", "value")
	if s.Tags["key"] != "value" {
		t.Fatalf("expected tag key=value, got %q", s.Tags["key"])
	}
}

func TestSpan_SetError(t *testing.T) {
	s := &Span{Tags: make(map[string]string)}
	s.SetError(errors.New("something broke"))

	if s.Status != StatusError {
		t.Fatal("expected StatusError")
	}
	if s.Tags["error"] != "true" {
		t.Fatal("expected error tag set to true")
	}
	if s.Tags["error.message"] != "something broke" {
		t.Fatalf("expected error message, got %q", s.Tags["error.message"])
	}
}

func TestSpan_AddEvent(t *testing.T) {
	s := &Span{Tags: make(map[string]string)}
	s.AddEvent("retry", map[string]string{"attempt": "2"})

	if len(s.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(s.Events))
	}
	if s.Events[0].Name != "retry" {
		t.Fatalf("expected event name 'retry', got %q", s.Events[0].Name)
	}
	if s.Events[0].Attributes["attempt"] != "2" {
		t.Fatalf("expected attribute attempt=2, got %q", s.Events[0].Attributes["attempt"])
	}
	if s.Events[0].TimestampUs == 0 {
		t.Fatal("expected non-zero timestamp")
	}
}

func TestSpan_ToProto(t *testing.T) {
	s := &Span{
		TraceID:      "0123456789abcdef0123456789abcdef",
		SpanID:       "0123456789abcdef",
		ParentSpanID: "fedcba9876543210",
		Operation:    "test-op",
		Service:      "test-svc",
		Kind:         KindServer,
		StartUs:      1000000,
		DurationUs:   5000,
		Status:       StatusOK,
		Tags:         map[string]string{"key": "val"},
		Events: []SpanEvent{
			{TimestampUs: 1001000, Name: "event1", Attributes: map[string]string{"a": "b"}},
		},
	}

	pb := s.ToProto()
	if pb.Operation != "test-op" {
		t.Fatalf("expected operation 'test-op', got %q", pb.Operation)
	}
	if pb.Service != "test-svc" {
		t.Fatalf("expected service 'test-svc', got %q", pb.Service)
	}
	if len(pb.Tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(pb.Tags))
	}
	if len(pb.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pb.Events))
	}
}

func TestSpanKind_String(t *testing.T) {
	tests := []struct {
		kind SpanKind
		want string
	}{
		{KindInternal, "internal"},
		{KindServer, "server"},
		{KindClient, "client"},
		{KindProducer, "producer"},
		{KindConsumer, "consumer"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("SpanKind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestSpanStatus_String(t *testing.T) {
	if StatusOK.String() != "ok" {
		t.Fatalf("expected 'ok', got %q", StatusOK.String())
	}
	if StatusError.String() != "error" {
		t.Fatalf("expected 'error', got %q", StatusError.String())
	}
}

func TestWithSpanContext(t *testing.T) {
	s := &Span{Tags: make(map[string]string)}
	sc := &SpanContext{
		TraceID:      "abc123",
		ParentSpanID: "def456",
		Sampled:      true,
	}
	WithSpanContext(sc)(s)

	if s.TraceID != "abc123" {
		t.Fatalf("expected TraceID 'abc123', got %q", s.TraceID)
	}
	if s.ParentSpanID != "def456" {
		t.Fatalf("expected ParentSpanID 'def456', got %q", s.ParentSpanID)
	}
}
