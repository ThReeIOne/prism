package sdk

import (
	"context"
	"log/slog"
	"sync"
	"time"

	prismpb "github.com/shengli/prism/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// BatchExporter buffers spans and sends them to the collector in batches.
type BatchExporter struct {
	collectorAddr string
	batchSize     int
	flushInterval time.Duration
	buffer        []*Span
	mu            sync.Mutex
	client        prismpb.CollectorServiceClient
	conn          *grpc.ClientConn
	done          chan struct{}
}

// NewBatchExporter creates a new exporter that batches spans for gRPC export.
func NewBatchExporter(addr string, batchSize int, flushInterval time.Duration) *BatchExporter {
	e := &BatchExporter{
		collectorAddr: addr,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		buffer:        make([]*Span, 0, batchSize),
		done:          make(chan struct{}),
	}

	go e.connect()
	go e.flushLoop()
	return e
}

func (e *BatchExporter) connect() {
	conn, err := grpc.NewClient(e.collectorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		slog.Error("failed to connect to collector", "error", err, "addr", e.collectorAddr)
		return
	}
	e.mu.Lock()
	e.conn = conn
	e.client = prismpb.NewCollectorServiceClient(conn)
	e.mu.Unlock()
}

// Enqueue adds a span to the buffer; triggers flush if the batch is full.
func (e *BatchExporter) Enqueue(span *Span) {
	e.mu.Lock()
	e.buffer = append(e.buffer, span)
	shouldFlush := len(e.buffer) >= e.batchSize
	e.mu.Unlock()

	if shouldFlush {
		go e.Flush()
	}
}

// Flush sends buffered spans to the collector.
func (e *BatchExporter) Flush() {
	e.mu.Lock()
	if len(e.buffer) == 0 {
		e.mu.Unlock()
		return
	}
	if e.client == nil {
		e.mu.Unlock()
		return
	}
	batch := e.buffer
	e.buffer = make([]*Span, 0, e.batchSize)
	client := e.client
	e.mu.Unlock()

	pbBatch := &prismpb.SpanBatch{
		Spans: make([]*prismpb.Span, len(batch)),
	}
	for i, s := range batch {
		pbBatch.Spans[i] = s.ToProto()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Report(ctx, pbBatch)
	if err != nil {
		slog.Warn("failed to export spans", "error", err, "count", len(batch))
		// Re-enqueue on failure (up to 10x batch limit)
		e.mu.Lock()
		if len(e.buffer)+len(batch) < e.batchSize*10 {
			e.buffer = append(batch, e.buffer...)
		}
		e.mu.Unlock()
	}
}

func (e *BatchExporter) flushLoop() {
	ticker := time.NewTicker(e.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.Flush()
		case <-e.done:
			return
		}
	}
}

// Shutdown flushes remaining spans and closes the connection.
func (e *BatchExporter) Shutdown() {
	close(e.done)
	e.Flush()
	e.mu.Lock()
	if e.conn != nil {
		e.conn.Close()
	}
	e.mu.Unlock()
}
