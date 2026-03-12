package propagation

import (
	"testing"

	"github.com/shengli/prism/sdk"
)

func TestKafka_InjectExtract(t *testing.T) {
	span := &sdk.Span{
		TraceID: "trace-abc",
		SpanID:  "span-123",
	}

	headers := InjectKafka(span)

	if len(headers) != 3 {
		t.Fatalf("expected 3 headers, got %d", len(headers))
	}

	sc := ExtractKafka(headers)
	if sc == nil {
		t.Fatal("expected non-nil SpanContext")
	}
	if sc.TraceID != "trace-abc" {
		t.Fatalf("expected TraceID 'trace-abc', got %q", sc.TraceID)
	}
	if sc.ParentSpanID != "span-123" {
		t.Fatalf("expected ParentSpanID 'span-123', got %q", sc.ParentSpanID)
	}
	if !sc.Sampled {
		t.Fatal("expected Sampled to be true")
	}
}

func TestKafka_InjectNil(t *testing.T) {
	headers := InjectKafka(nil)
	if headers != nil {
		t.Fatal("expected nil for nil span")
	}
}

func TestKafka_ExtractEmpty(t *testing.T) {
	sc := ExtractKafka(nil)
	if sc != nil {
		t.Fatal("expected nil for nil headers")
	}
}
