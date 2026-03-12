package propagation

import (
	"net/http"
	"testing"

	"github.com/shengli/prism/sdk"
)

func TestHTTP_InjectExtract(t *testing.T) {
	span := &sdk.Span{
		TraceID: "abc123",
		SpanID:  "def456",
	}

	header := http.Header{}
	Inject(span, header)

	if header.Get(HeaderTraceID) != "abc123" {
		t.Fatalf("expected trace_id 'abc123', got %q", header.Get(HeaderTraceID))
	}
	if header.Get(HeaderSpanID) != "def456" {
		t.Fatalf("expected span_id 'def456', got %q", header.Get(HeaderSpanID))
	}
	if header.Get(HeaderSampled) != "1" {
		t.Fatalf("expected sampled '1', got %q", header.Get(HeaderSampled))
	}

	sc := Extract(header)
	if sc == nil {
		t.Fatal("expected non-nil SpanContext")
	}
	if sc.TraceID != "abc123" {
		t.Fatalf("expected TraceID 'abc123', got %q", sc.TraceID)
	}
	if sc.ParentSpanID != "def456" {
		t.Fatalf("expected ParentSpanID 'def456', got %q", sc.ParentSpanID)
	}
	if !sc.Sampled {
		t.Fatal("expected Sampled to be true")
	}
}

func TestHTTP_InjectNilSpan(t *testing.T) {
	header := http.Header{}
	Inject(nil, header)
	if header.Get(HeaderTraceID) != "" {
		t.Fatal("expected no headers for nil span")
	}
}

func TestHTTP_ExtractEmpty(t *testing.T) {
	header := http.Header{}
	sc := Extract(header)
	if sc != nil {
		t.Fatal("expected nil for empty headers")
	}
}
