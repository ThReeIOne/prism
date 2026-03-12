package propagation

import "github.com/shengli/prism/sdk"

const (
	KafkaTraceID = "x-prism-trace-id"
	KafkaSpanID  = "x-prism-span-id"
	KafkaSampled = "x-prism-sampled"
)

// KafkaHeader represents a key-value header pair in a Kafka message.
type KafkaHeader struct {
	Key   string
	Value []byte
}

// InjectKafka returns headers that carry trace context for Kafka messages.
func InjectKafka(span *sdk.Span) []KafkaHeader {
	if span == nil {
		return nil
	}
	return []KafkaHeader{
		{Key: KafkaTraceID, Value: []byte(span.TraceID)},
		{Key: KafkaSpanID, Value: []byte(span.SpanID)},
		{Key: KafkaSampled, Value: []byte("1")},
	}
}

// ExtractKafka reads trace context from Kafka message headers.
func ExtractKafka(headers []KafkaHeader) *sdk.SpanContext {
	sc := &sdk.SpanContext{}
	for _, h := range headers {
		switch h.Key {
		case KafkaTraceID:
			sc.TraceID = string(h.Value)
		case KafkaSpanID:
			sc.ParentSpanID = string(h.Value)
		case KafkaSampled:
			sc.Sampled = string(h.Value) == "1"
		}
	}
	if sc.TraceID == "" {
		return nil
	}
	return sc
}
