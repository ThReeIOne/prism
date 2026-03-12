package sdk

import (
	"encoding/hex"
	"fmt"
	"time"

	prismpb "github.com/shengli/prism/proto/gen"
)

// SpanKind represents the role of a span in a trace.
type SpanKind int

const (
	KindInternal SpanKind = iota
	KindServer
	KindClient
	KindProducer
	KindConsumer
)

func (k SpanKind) String() string {
	switch k {
	case KindServer:
		return "server"
	case KindClient:
		return "client"
	case KindProducer:
		return "producer"
	case KindConsumer:
		return "consumer"
	default:
		return "internal"
	}
}

// SpanStatus represents the outcome of a span.
type SpanStatus int

const (
	StatusOK    SpanStatus = 0
	StatusError SpanStatus = 1
)

func (s SpanStatus) String() string {
	if s == StatusError {
		return "error"
	}
	return "ok"
}

// SpanEvent represents a timed event within a span.
type SpanEvent struct {
	TimestampUs uint64
	Name        string
	Attributes  map[string]string
}

// Span represents a single unit of work in a distributed trace.
type Span struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	Operation    string
	Service      string
	Kind         SpanKind
	StartUs      uint64
	DurationUs   uint64
	Status       SpanStatus
	Tags         map[string]string
	Events       []SpanEvent
}

// SetTag sets a tag on the span.
func (s *Span) SetTag(key, value string) {
	s.Tags[key] = value
}

// SetError marks the span as an error and records the error message.
func (s *Span) SetError(err error) {
	s.Status = StatusError
	s.Tags["error"] = "true"
	s.Tags["error.message"] = err.Error()
}

// AddEvent adds a timed event to the span.
func (s *Span) AddEvent(name string, attrs map[string]string) {
	s.Events = append(s.Events, SpanEvent{
		TimestampUs: uint64(time.Now().UnixMicro()),
		Name:        name,
		Attributes:  attrs,
	})
}

// ToProto converts the span to its protobuf representation.
func (s *Span) ToProto() *prismpb.Span {
	traceIDBytes, _ := hex.DecodeString(s.TraceID)
	spanIDBytes, _ := hex.DecodeString(s.SpanID)
	parentBytes, _ := hex.DecodeString(s.ParentSpanID)

	tags := make([]*prismpb.KeyValue, 0, len(s.Tags))
	for k, v := range s.Tags {
		tags = append(tags, &prismpb.KeyValue{Key: k, Value: v})
	}

	events := make([]*prismpb.SpanEvent, 0, len(s.Events))
	for _, e := range s.Events {
		attrs := make([]*prismpb.KeyValue, 0, len(e.Attributes))
		for k, v := range e.Attributes {
			attrs = append(attrs, &prismpb.KeyValue{Key: k, Value: v})
		}
		events = append(events, &prismpb.SpanEvent{
			TimestampUs: e.TimestampUs,
			Name:        e.Name,
			Attributes:  attrs,
		})
	}

	return &prismpb.Span{
		TraceId:      traceIDBytes,
		SpanId:       spanIDBytes,
		ParentSpanId: parentBytes,
		Operation:    s.Operation,
		Service:      s.Service,
		Kind:         prismpb.SpanKind(s.Kind),
		StartUs:      s.StartUs,
		DurationUs:   s.DurationUs,
		Status:       prismpb.StatusCode(s.Status),
		Tags:         tags,
		Events:       events,
	}
}

// SpanContext holds the trace propagation context extracted from carriers.
type SpanContext struct {
	TraceID      string
	ParentSpanID string
	Sampled      bool
}

// SpanOption configures a Span at creation time.
type SpanOption func(*Span)

// WithKind sets the span kind.
func WithKind(kind SpanKind) SpanOption {
	return func(s *Span) {
		s.Kind = kind
	}
}

// WithTag sets a tag at span creation.
func WithTag(key, value string) SpanOption {
	return func(s *Span) {
		s.Tags[key] = value
	}
}

// WithSpanContext applies an extracted propagation context to the span.
func WithSpanContext(sc *SpanContext) SpanOption {
	return func(s *Span) {
		if sc != nil {
			s.TraceID = sc.TraceID
			s.ParentSpanID = sc.ParentSpanID
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("...(%d more)", len(s)-max)
}
