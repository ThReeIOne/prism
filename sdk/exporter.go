package sdk

import (
	"context"
	"log/slog"
	"sync"
	"time"

	prismpb "github.com/shengli/prism/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// BatchExporter buffers spans and sends them to the collector in batches.
type BatchExporter struct {
	collectorAddr string
	batchSize     int
	flushInterval time.Duration
	ingestToken   string // optional bearer token for gRPC metadata
	buffer        []*Span
	mu            sync.Mutex
	client        prismpb.CollectorServiceClient
	conn          *grpc.ClientConn
	ready         chan struct{} // closed when gRPC connection is ready
	done          chan struct{}
	shutdownOnce  sync.Once
	wg            sync.WaitGroup // tracks flushLoop goroutine
}

// NewBatchExporter creates a new exporter that batches spans for gRPC export.
func NewBatchExporter(addr string, batchSize int, flushInterval time.Duration) *BatchExporter {
	return newBatchExporter(addr, batchSize, flushInterval, "")
}

// newBatchExporter is the internal constructor that also accepts an ingest token.
func newBatchExporter(addr string, batchSize int, flushInterval time.Duration, token string) *BatchExporter {
	e := &BatchExporter{
		collectorAddr: addr,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		ingestToken:   token,
		buffer:        make([]*Span, 0, batchSize),
		ready:         make(chan struct{}),
		done:          make(chan struct{}),
	}

	go e.connect()
	e.wg.Add(1)
	go e.flushLoop()
	return e
}

func (e *BatchExporter) connect() {
	conn, err := grpc.NewClient(e.collectorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		slog.Error("failed to connect to collector", "error", err, "addr", e.collectorAddr)
		close(e.ready) // unblock Flush so it doesn't wait 3s on every call
		return
	}
	e.mu.Lock()
	e.conn = conn
	e.client = prismpb.NewCollectorServiceClient(conn)
	e.mu.Unlock()
	close(e.ready)
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
	// Wait for connection to be ready (with timeout)
	select {
	case <-e.ready:
	case <-time.After(3 * time.Second):
		slog.Warn("flush: collector connection not ready, retaining spans in buffer")
		return
	}

	e.mu.Lock()
	if len(e.buffer) == 0 {
		e.mu.Unlock()
		return
	}
	batch := e.buffer
	e.buffer = make([]*Span, 0, e.batchSize)
	client := e.client
	token := e.ingestToken
	e.mu.Unlock()

	if client == nil {
		// Connection failed permanently, re-enqueue
		e.mu.Lock()
		e.buffer = append(batch, e.buffer...)
		e.mu.Unlock()
		return
	}

	pbBatch := &prismpb.SpanBatch{
		Spans: make([]*prismpb.Span, len(batch)),
	}
	for i, s := range batch {
		pbBatch.Spans[i] = s.ToProto()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attach bearer token to outgoing gRPC metadata if configured.
	if token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
	}

	_, err := client.Report(ctx, pbBatch)
	if err != nil {
		slog.Warn("failed to export spans", "error", err, "count", len(batch))
		// Re-enqueue on failure (up to 10x batch limit to prevent OOM)
		e.mu.Lock()
		if len(e.buffer)+len(batch) < e.batchSize*10 {
			e.buffer = append(batch, e.buffer...)
		} else {
			slog.Warn("export buffer full, dropping spans", "count", len(batch))
		}
		e.mu.Unlock()
	}
}

func (e *BatchExporter) flushLoop() {
	defer e.wg.Done()
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
	e.shutdownOnce.Do(func() {
		close(e.done)
		e.wg.Wait() // wait for flushLoop to exit before final flush
		e.Flush()
		e.mu.Lock()
		if e.conn != nil {
			e.conn.Close()
		}
		e.mu.Unlock()
	})
}
