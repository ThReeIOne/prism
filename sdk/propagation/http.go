package propagation

import (
	"net/http"

	"github.com/shengli/prism/sdk"
)

const (
	HeaderTraceID = "X-Prism-Trace-Id"
	HeaderSpanID  = "X-Prism-Span-Id"
	HeaderSampled = "X-Prism-Sampled"
)

// Inject writes trace context into HTTP headers.
func Inject(span *sdk.Span, header http.Header) {
	if span == nil {
		return
	}
	header.Set(HeaderTraceID, span.TraceID)
	header.Set(HeaderSpanID, span.SpanID)
	header.Set(HeaderSampled, "1")
}

// Extract reads trace context from HTTP headers.
func Extract(header http.Header) *sdk.SpanContext {
	traceID := header.Get(HeaderTraceID)
	if traceID == "" {
		return nil
	}
	return &sdk.SpanContext{
		TraceID:      traceID,
		ParentSpanID: header.Get(HeaderSpanID),
		Sampled:      header.Get(HeaderSampled) == "1",
	}
}
