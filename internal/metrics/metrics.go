package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Collector metrics
var (
	SpansReceived = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "prism",
		Subsystem: "collector",
		Name:      "spans_received_total",
		Help:      "Total number of spans received by the collector.",
	})

	SpansDropped = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "prism",
		Subsystem: "collector",
		Name:      "spans_dropped_total",
		Help:      "Total number of spans dropped (flush failure or buffer full).",
	})

	FlushDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "prism",
		Subsystem: "collector",
		Name:      "flush_duration_seconds",
		Help:      "Time spent flushing spans to ClickHouse.",
		Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	})

	FlushBatchSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "prism",
		Subsystem: "collector",
		Name:      "flush_batch_size",
		Help:      "Number of spans per flush batch.",
		Buckets:   []float64{10, 50, 100, 500, 1000, 2000, 5000, 10000},
	})

	BufferSize = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "prism",
		Subsystem: "collector",
		Name:      "buffer_size",
		Help:      "Current number of spans in the write buffer.",
	})

	// BufferCapacity is the configured max buffer size, useful for calculating
	// buffer utilisation: buffer_size / buffer_capacity.
	BufferCapacity = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "prism",
		Subsystem: "collector",
		Name:      "buffer_capacity",
		Help:      "Configured maximum buffer size (max spans before backpressure).",
	})

	FlushErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "prism",
		Subsystem: "collector",
		Name:      "flush_errors_total",
		Help:      "Total number of flush errors.",
	})
)

// Query metrics
var (
	QueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "prism",
		Subsystem: "query",
		Name:      "duration_seconds",
		Help:      "Query API request duration in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"endpoint", "status"})

	QueryRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "prism",
		Subsystem: "query",
		Name:      "requests_total",
		Help:      "Total number of query API requests.",
	}, []string{"endpoint", "status"})
)

// SDK export metrics (reported by collector on receive)
var (
	SDKReportBatchSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "prism",
		Subsystem: "sdk",
		Name:      "report_batch_size",
		Help:      "Number of spans per SDK report batch.",
		Buckets:   []float64{1, 10, 50, 100, 256, 512, 1024, 2048},
	})
)
