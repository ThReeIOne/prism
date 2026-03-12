package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// ClickHouseStorage implements Storage backed by ClickHouse.
type ClickHouseStorage struct {
	conn driver.Conn
}

// ClickHouseConfig holds connection configuration.
type ClickHouseConfig struct {
	Addrs    []string
	Database string
	Username string
	Password string
}

// NewClickHouseStorage creates a new ClickHouse storage.
func NewClickHouseStorage(cfg ClickHouseConfig) (*ClickHouseStorage, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: cfg.Addrs,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		MaxOpenConns: 10,
		MaxIdleConns: 5,
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}
	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}
	return &ClickHouseStorage{conn: conn}, nil
}

func (s *ClickHouseStorage) BatchInsert(ctx context.Context, spans []SpanRecord) error {
	batch, err := s.conn.PrepareBatch(ctx, `
		INSERT INTO spans (trace_id, span_id, parent_span_id, operation, service, kind, start_us, duration_us, status, tags, events)
	`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	for _, sp := range spans {
		if err := batch.Append(
			sp.TraceID,
			sp.SpanID,
			sp.ParentSpanID,
			sp.Operation,
			sp.Service,
			sp.Kind,
			sp.StartUs,
			sp.DurationUs,
			sp.Status,
			sp.Tags,
			sp.Events,
		); err != nil {
			slog.Warn("batch append failed", "error", err)
			continue
		}
	}
	return batch.Send()
}

func (s *ClickHouseStorage) GetTrace(ctx context.Context, traceID string) (*TraceResult, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT trace_id, span_id, parent_span_id, operation, service, kind,
		       start_us, duration_us, status, tags, events
		FROM spans
		WHERE trace_id = ?
		ORDER BY start_us
	`, traceID)
	if err != nil {
		return nil, fmt.Errorf("query trace: %w", err)
	}
	defer rows.Close()

	result := &TraceResult{TraceID: traceID}
	for rows.Next() {
		var sp SpanRecord
		if err := rows.Scan(
			&sp.TraceID, &sp.SpanID, &sp.ParentSpanID,
			&sp.Operation, &sp.Service, &sp.Kind,
			&sp.StartUs, &sp.DurationUs, &sp.Status,
			&sp.Tags, &sp.Events,
		); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		result.Spans = append(result.Spans, sp)
	}
	if len(result.Spans) == 0 {
		return nil, nil
	}
	return result, nil
}

func (s *ClickHouseStorage) SearchTraces(ctx context.Context, params TraceSearchParams) ([]TraceResult, error) {
	var conditions []string
	var args []any

	if params.Service != "" {
		conditions = append(conditions, "service = ?")
		args = append(args, params.Service)
	}
	if params.Operation != "" {
		conditions = append(conditions, "operation = ?")
		args = append(args, params.Operation)
	}
	if params.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, params.Status)
	}
	if params.MinDuration > 0 {
		conditions = append(conditions, "duration_us >= ?")
		args = append(args, uint64(params.MinDuration.Microseconds()))
	}
	if params.MaxDuration > 0 {
		conditions = append(conditions, "duration_us <= ?")
		args = append(args, uint64(params.MaxDuration.Microseconds()))
	}
	if !params.Start.IsZero() {
		conditions = append(conditions, "start_us >= ?")
		args = append(args, uint64(params.Start.UnixMicro()))
	}
	if !params.End.IsZero() {
		conditions = append(conditions, "start_us <= ?")
		args = append(args, uint64(params.End.UnixMicro()))
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	// First find matching trace IDs
	query := fmt.Sprintf(`
		SELECT DISTINCT trace_id
		FROM spans
		%s
		ORDER BY max(start_us) DESC
		LIMIT %d
	`, where, limit)

	rows, err := s.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search traces: %w", err)
	}
	defer rows.Close()

	var traceIDs []string
	for rows.Next() {
		var tid string
		if err := rows.Scan(&tid); err != nil {
			return nil, err
		}
		traceIDs = append(traceIDs, tid)
	}

	// Fetch full traces
	var results []TraceResult
	for _, tid := range traceIDs {
		trace, err := s.GetTrace(ctx, tid)
		if err != nil {
			slog.Warn("failed to fetch trace", "trace_id", tid, "error", err)
			continue
		}
		if trace != nil {
			results = append(results, *trace)
		}
	}
	return results, nil
}

func (s *ClickHouseStorage) GetServices(ctx context.Context) ([]ServiceInfo, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT
			service,
			count() / 3600 AS qps,
			countIf(status = 'error') / greatest(count(), 1) AS error_rate,
			quantile(0.99)(duration_us) / 1000 AS p99_ms
		FROM spans
		WHERE start_us >= ?
		GROUP BY service
		ORDER BY service
	`, uint64(time.Now().Add(-1*time.Hour).UnixMicro()))
	if err != nil {
		return nil, fmt.Errorf("query services: %w", err)
	}
	defer rows.Close()

	var services []ServiceInfo
	for rows.Next() {
		var svc ServiceInfo
		if err := rows.Scan(&svc.Name, &svc.QPS, &svc.ErrorRate, &svc.P99Latency); err != nil {
			return nil, err
		}
		services = append(services, svc)
	}
	return services, nil
}

func (s *ClickHouseStorage) GetOperations(ctx context.Context, service string) ([]OperationInfo, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT
			operation,
			sum(call_count) AS call_count,
			sum(error_count) AS error_count,
			total_duration_us / greatest(call_count, 1) / 1000 AS avg_ms,
			max(max_duration_us) / 1000 AS max_ms
		FROM service_operations_mv
		WHERE service = ?
		GROUP BY operation, total_duration_us, call_count
		ORDER BY call_count DESC
	`, service)
	if err != nil {
		return nil, fmt.Errorf("query operations: %w", err)
	}
	defer rows.Close()

	var ops []OperationInfo
	for rows.Next() {
		var op OperationInfo
		if err := rows.Scan(&op.Operation, &op.CallCount, &op.ErrorCount, &op.AvgLatency, &op.MaxLatency); err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, nil
}

func (s *ClickHouseStorage) GetDependencies(ctx context.Context, start, end time.Time) ([]DependencyEdge, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT
			parent_service,
			child_service,
			sum(call_count) AS call_count,
			sum(error_count) AS error_count,
			avg(avg_duration_us) / 1000 AS avg_ms
		FROM service_dependencies_mv
		WHERE date >= ? AND date <= ?
		GROUP BY parent_service, child_service
		ORDER BY call_count DESC
	`, start, end)
	if err != nil {
		return nil, fmt.Errorf("query dependencies: %w", err)
	}
	defer rows.Close()

	var deps []DependencyEdge
	for rows.Next() {
		var dep DependencyEdge
		if err := rows.Scan(&dep.Parent, &dep.Child, &dep.CallCount, &dep.ErrorCount, &dep.AvgLatency); err != nil {
			return nil, err
		}
		deps = append(deps, dep)
	}
	return deps, nil
}

func (s *ClickHouseStorage) GetLatencyStats(ctx context.Context, service, operation string, start, end time.Time, granularity time.Duration) ([]LatencyPoint, error) {
	intervalSec := int(granularity.Seconds())
	if intervalSec < 1 {
		intervalSec = 60
	}

	var conditions []string
	var args []any

	conditions = append(conditions, "start_us >= ?", "start_us <= ?")
	args = append(args, uint64(start.UnixMicro()), uint64(end.UnixMicro()))

	if service != "" {
		conditions = append(conditions, "service = ?")
		args = append(args, service)
	}
	if operation != "" {
		conditions = append(conditions, "operation = ?")
		args = append(args, operation)
	}

	where := strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT
			toStartOfInterval(fromUnixTimestamp64Micro(start_us), INTERVAL %d SECOND) AS ts,
			quantile(0.50)(duration_us) / 1000 AS p50,
			quantile(0.90)(duration_us) / 1000 AS p90,
			quantile(0.99)(duration_us) / 1000 AS p99,
			max(duration_us) / 1000 AS max_ms
		FROM spans
		WHERE %s
		GROUP BY ts
		ORDER BY ts
	`, intervalSec, where)

	rows, err := s.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query latency: %w", err)
	}
	defer rows.Close()

	var points []LatencyPoint
	for rows.Next() {
		var pt LatencyPoint
		if err := rows.Scan(&pt.Timestamp, &pt.P50, &pt.P90, &pt.P99, &pt.Max); err != nil {
			return nil, err
		}
		points = append(points, pt)
	}
	return points, nil
}

func (s *ClickHouseStorage) GetThroughputStats(ctx context.Context, service string, start, end time.Time, granularity time.Duration) ([]ThroughputPoint, error) {
	intervalSec := int(granularity.Seconds())
	if intervalSec < 1 {
		intervalSec = 60
	}

	var conditions []string
	var args []any

	conditions = append(conditions, "start_us >= ?", "start_us <= ?")
	args = append(args, uint64(start.UnixMicro()), uint64(end.UnixMicro()))

	if service != "" {
		conditions = append(conditions, "service = ?")
		args = append(args, service)
	}

	where := strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT
			toStartOfInterval(fromUnixTimestamp64Micro(start_us), INTERVAL %d SECOND) AS ts,
			count() AS total,
			countIf(status = 'error') AS errors
		FROM spans
		WHERE %s
		GROUP BY ts
		ORDER BY ts
	`, intervalSec, where)

	rows, err := s.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query throughput: %w", err)
	}
	defer rows.Close()

	var points []ThroughputPoint
	for rows.Next() {
		var pt ThroughputPoint
		if err := rows.Scan(&pt.Timestamp, &pt.Total, &pt.Errors); err != nil {
			return nil, err
		}
		if pt.Total > 0 {
			pt.ErrorRate = float64(pt.Errors) / float64(pt.Total)
		}
		points = append(points, pt)
	}
	return points, nil
}

func (s *ClickHouseStorage) Close() error {
	return s.conn.Close()
}
