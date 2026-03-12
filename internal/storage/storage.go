package storage

import (
	"context"
	"time"
)

// SpanRecord represents a stored span.
type SpanRecord struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	Operation    string
	Service      string
	Kind         string
	StartUs      uint64
	DurationUs   uint64
	Status       string
	Tags         map[string]string
	Events       string // JSON-encoded events
	Date         time.Time
}

// TraceResult is a collection of spans forming a complete trace.
type TraceResult struct {
	TraceID string
	Spans   []SpanRecord
}

// ServiceInfo provides summary info about a service.
type ServiceInfo struct {
	Name       string  `json:"name"`
	QPS        float64 `json:"qps"`
	ErrorRate  float64 `json:"error_rate"`
	P99Latency float64 `json:"p99_latency_ms"`
}

// OperationInfo provides summary info about an operation.
type OperationInfo struct {
	Operation  string  `json:"operation"`
	CallCount  uint64  `json:"call_count"`
	ErrorCount uint64  `json:"error_count"`
	AvgLatency float64 `json:"avg_latency_ms"`
	MaxLatency float64 `json:"max_latency_ms"`
}

// DependencyEdge represents a call relationship between two services.
type DependencyEdge struct {
	Parent     string  `json:"parent"`
	Child      string  `json:"child"`
	CallCount  uint64  `json:"call_count"`
	ErrorCount uint64  `json:"error_count"`
	AvgLatency float64 `json:"avg_latency_ms"`
}

// LatencyPoint is a single data point in a latency time series.
type LatencyPoint struct {
	Timestamp time.Time `json:"timestamp"`
	P50       float64   `json:"p50"`
	P90       float64   `json:"p90"`
	P99       float64   `json:"p99"`
	Max       float64   `json:"max"`
}

// ThroughputPoint is a single data point in a throughput time series.
type ThroughputPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Total     uint64    `json:"total"`
	Errors    uint64    `json:"errors"`
	ErrorRate float64   `json:"error_rate"`
}

// TraceSearchParams holds filters for searching traces.
type TraceSearchParams struct {
	Service     string
	Operation   string
	MinDuration time.Duration
	MaxDuration time.Duration
	Status      string
	Tags        map[string]string
	Start       time.Time
	End         time.Time
	Limit       int
}

// Storage is the interface for trace data persistence.
type Storage interface {
	// BatchInsert writes a batch of spans.
	BatchInsert(ctx context.Context, spans []SpanRecord) error

	// GetTrace retrieves all spans for a given trace ID.
	GetTrace(ctx context.Context, traceID string) (*TraceResult, error)

	// SearchTraces searches for traces matching the given criteria.
	SearchTraces(ctx context.Context, params TraceSearchParams) ([]TraceResult, error)

	// GetServices returns all known services with recent statistics.
	GetServices(ctx context.Context) ([]ServiceInfo, error)

	// GetOperations returns all operations for a given service.
	GetOperations(ctx context.Context, service string) ([]OperationInfo, error)

	// GetDependencies returns service dependency edges within a time range.
	GetDependencies(ctx context.Context, start, end time.Time) ([]DependencyEdge, error)

	// GetLatencyStats returns latency time series data.
	GetLatencyStats(ctx context.Context, service, operation string, start, end time.Time, granularity time.Duration) ([]LatencyPoint, error)

	// GetThroughputStats returns throughput time series data.
	GetThroughputStats(ctx context.Context, service string, start, end time.Time, granularity time.Duration) ([]ThroughputPoint, error)

	// Close shuts down the storage connection.
	Close() error
}
