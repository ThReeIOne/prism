# Prism — 分布式链路追踪系统

> 一束请求穿过多个服务，Prism 把它折射成清晰可见的完整链路

## 一、项目概述

### 定位

Prism 是一个轻量级分布式链路追踪系统，自动采集请求在多个服务间的调用链路、耗时、错误信息，提供链路搜索、服务依赖拓扑图、性能瓶颈定位能力。

### 解决的问题

- 一个请求经过 API → DB → Redis → Kafka → 下游服务，某环节慢了或出错了，定位困难
- 日志散落在多个服务，排查问题需要跨机器 grep，靠时间戳人肉拼凑
- 不知道服务间的真实调用关系和依赖
- 性能优化没有数据支撑，不知道瓶颈在哪

### 不做什么

- 不做 Metrics 聚合（Prometheus 做得更好）
- 不做 Log 聚合（ELK/Loki 做得更好）
- 不做 APM 全家桶，专注 Tracing 这一件事

## 二、核心概念

```
Trace（一次完整请求的生命周期）
│
├── Span A: API Gateway         service=gateway   duration=350ms
│   │
│   ├── Span B: User Service    service=user-svc   duration=45ms
│   │   └── Span C: PG Query   service=user-svc   duration=12ms
│   │
│   ├── Span D: Order Service   service=order-svc  duration=200ms
│   │   ├── Span E: Redis GET   service=order-svc  duration=2ms
│   │   └── Span F: Kafka Send  service=order-svc  duration=15ms
│   │
│   └── Span G: Notify Service  service=notify-svc duration=80ms
│       └── Span H: HTTP POST   service=notify-svc duration=75ms

每个 Span = 一次操作的时间切片
- trace_id:  贯穿所有服务，唯一标识这次请求
- span_id:   当前操作的唯一 ID
- parent_id: 谁触发了我
```

## 三、技术栈

| 组件 | 选型 | 理由 |
|------|------|------|
| 语言 | **Go 1.22+** | Jaeger/Zipkin Collector 都是 Go，高吞吐低延迟 |
| SDK | **Go SDK (首版)** | 基于 `context.Context` 传播，零分配优化 |
| 采集协议 | **Protobuf over gRPC** | 比 JSON 小 5-10x，SDK → Collector 高效传输 |
| Collector | **自研 Go 服务** | 接收、采样、攒批、写入 |
| 存储 | **ClickHouse** | 列式存储，时序查询极快，自带 TTL 过期，压缩比高 |
| 索引辅助 | **ClickHouse 物化视图** | 按 service/operation 预聚合 |
| 依赖图缓存 | **Redis** | 服务调用关系快速读取 |
| Query API | **Go + chi** | 给前端/告警系统查询用 |
| 前端 | **React (可选)** | Trace 时间线瀑布图 + 拓扑图 |

## 四、整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Application Services                         │
│                                                                     │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐       │
│  │ Service A  │  │ Service B  │  │ Service C  │  │ Service D  │       │
│  │  ┌──────┐ │  │  ┌──────┐ │  │  ┌──────┐ │  │  ┌──────┐ │       │
│  │  │ SDK  │ │  │  │ SDK  │ │  │  │ SDK  │ │  │  │ SDK  │ │       │
│  │  └──┬───┘ │  │  └──┬───┘ │  │  └──┬───┘ │  │  └──┬───┘ │       │
│  └─────┼─────┘  └─────┼─────┘  └─────┼─────┘  └─────┼─────┘       │
│        │              │              │              │                │
└────────┼──────────────┼──────────────┼──────────────┼───────────────┘
         │              │              │              │
         │         gRPC 批量上报        │              │
         ▼              ▼              ▼              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Collector Cluster (无状态，可水平扩展)           │
│                                                                     │
│  ┌────────────────────────────────────────────────────┐             │
│  │                  Pipeline                          │             │
│  │                                                    │             │
│  │  Receive → Validate → Sample → Enrich → Batch     │             │
│  │                                           │        │             │
│  └───────────────────────────────────────────┼────────┘             │
│                                              │                      │
│                                    ┌─────────▼──────────┐           │
│                                    │   Write Buffer     │           │
│                                    │   (5000条 or 5s)   │           │
│                                    └─────────┬──────────┘           │
└──────────────────────────────────────────────┼──────────────────────┘
                                               │ Batch Insert
                              ┌────────────────▼────────────────┐
                              │          ClickHouse             │
                              │                                 │
                              │  ┌───────────────────────┐      │
                              │  │  spans 表              │      │  主表，按 date 分区
                              │  │  ORDER BY (trace_id)  │      │
                              │  └───────────────────────┘      │
                              │  ┌───────────────────────┐      │
                              │  │  service_operations    │      │  物化视图：按 service 聚合
                              │  │  (Materialized View)  │      │
                              │  └───────────────────────┘      │
                              │  ┌───────────────────────┐      │
                              │  │  service_dependencies  │      │  物化视图：调用关系
                              │  │  (Materialized View)  │      │
                              │  └───────────────────────┘      │
                              └────────────────┬────────────────┘
                                               │
                              ┌────────────────▼────────────────┐
                              │        Query Service            │
                              │                                 │
                              │  GET /api/v1/traces/:id         │  ← 查完整 trace
                              │  GET /api/v1/traces             │  ← 搜索 trace
                              │  GET /api/v1/services           │  ← 服务列表
                              │  GET /api/v1/services/:name/ops │  ← 操作列表
                              │  GET /api/v1/dependencies       │  ← 服务拓扑
                              │  GET /api/v1/stats              │  ← 延迟/错误率统计
                              └─────────────────────────────────┘
```

## 五、数据模型

### 5.1 Span 定义 (Protobuf)

```protobuf
syntax = "proto3";
package prism;

message Span {
    bytes  trace_id       = 1;  // 16 bytes (128-bit)
    bytes  span_id        = 2;  // 8 bytes (64-bit)
    bytes  parent_span_id = 3;  // 8 bytes, empty for root span
    string operation      = 4;  // "POST /api/orders"
    string service        = 5;  // "order-svc"
    SpanKind kind         = 6;
    uint64 start_us       = 7;  // 微秒时间戳
    uint64 duration_us    = 8;
    StatusCode status     = 9;
    repeated KeyValue tags   = 10;
    repeated SpanEvent events = 11;
}

enum SpanKind {
    INTERNAL = 0;
    SERVER   = 1;  // 被调用方（收到请求）
    CLIENT   = 2;  // 调用方（发出请求）
    PRODUCER = 3;  // 消息生产者
    CONSUMER = 4;  // 消息消费者
}

enum StatusCode {
    OK    = 0;
    ERROR = 1;
}

message KeyValue {
    string key   = 1;
    string value = 2;
}

message SpanEvent {
    uint64 timestamp_us = 1;
    string name         = 2;  // "exception", "retry"
    repeated KeyValue attributes = 3;
}

// SDK 批量上报用
message SpanBatch {
    repeated Span spans = 1;
}
```

### 5.2 ClickHouse 建表

```sql
-- 主表：Span 存储
CREATE TABLE spans (
    trace_id         FixedString(32),       -- hex 编码的 128-bit
    span_id          FixedString(16),       -- hex 编码的 64-bit
    parent_span_id   String DEFAULT '',
    operation        LowCardinality(String),
    service          LowCardinality(String),
    kind             LowCardinality(String),
    start_us         UInt64,
    duration_us      UInt64,
    status           LowCardinality(String), -- "ok" | "error"
    tags             Map(String, String),
    events           String,                 -- JSON array
    date             Date DEFAULT toDate(fromUnixTimestamp64Micro(start_us))
)
ENGINE = MergeTree()
PARTITION BY date
ORDER BY (trace_id, start_us)
TTL date + INTERVAL 14 DAY
SETTINGS index_granularity = 8192;

-- 二级索引：按 service + operation 搜索
ALTER TABLE spans ADD INDEX idx_service service TYPE set(100) GRANULARITY 4;
ALTER TABLE spans ADD INDEX idx_operation operation TYPE tokenbf_v1(256, 2, 0) GRANULARITY 4;

-- 物化视图：service 维度聚合（自动维护）
CREATE MATERIALIZED VIEW service_operations_mv
ENGINE = SummingMergeTree()
ORDER BY (service, operation, date)
AS SELECT
    service,
    operation,
    date,
    count()              AS call_count,
    countIf(status = 'error') AS error_count,
    sum(duration_us)     AS total_duration_us,
    max(duration_us)     AS max_duration_us
FROM spans
GROUP BY service, operation, date;

-- 物化视图：服务间调用关系
CREATE MATERIALIZED VIEW service_dependencies_mv
ENGINE = SummingMergeTree()
ORDER BY (parent_service, child_service, date)
AS SELECT
    s1.service AS parent_service,
    s2.service AS child_service,
    s2.date    AS date,
    count()    AS call_count,
    countIf(s2.status = 'error') AS error_count,
    avg(s2.duration_us) AS avg_duration_us
FROM spans s1
INNER JOIN spans s2
    ON s1.trace_id = s2.trace_id
    AND s1.span_id = s2.parent_span_id
WHERE s1.service != s2.service
GROUP BY parent_service, child_service, date;
```

## 六、核心模块设计

### 6.1 SDK — Tracer 核心

```go
package prism

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "sync"
    "time"
)

// 使用 context key 传播 Span
type spanContextKey struct{}

// Tracer 是 SDK 入口
type Tracer struct {
    service  string
    exporter *BatchExporter
    sampler  Sampler
}

func NewTracer(service string, opts ...Option) *Tracer {
    cfg := defaultConfig()
    for _, o := range opts {
        o(cfg)
    }
    return &Tracer{
        service:  service,
        exporter: NewBatchExporter(cfg.CollectorAddr, cfg.BatchSize, cfg.FlushInterval),
        sampler:  cfg.Sampler,
    }
}

// StartSpan 开始一个新的 Span
func (t *Tracer) StartSpan(ctx context.Context, operation string, opts ...SpanOption) (context.Context, *Span) {
    parent := SpanFromContext(ctx)

    span := &Span{
        TraceID:   t.resolveTraceID(parent),
        SpanID:    generateID(8),
        Operation: operation,
        Service:   t.service,
        Kind:      KindInternal,
        StartUs:   uint64(time.Now().UnixMicro()),
        Status:    StatusOK,
        Tags:      make(map[string]string),
    }

    if parent != nil {
        span.ParentSpanID = parent.SpanID
    }

    for _, o := range opts {
        o(span)
    }

    return context.WithValue(ctx, spanContextKey{}, span), span
}

// FinishSpan 结束 Span 并提交给 Exporter
func (t *Tracer) FinishSpan(span *Span) {
    span.DurationUs = uint64(time.Now().UnixMicro()) - span.StartUs

    if t.sampler.ShouldSample(span) {
        t.exporter.Enqueue(span)
    }
}

func (t *Tracer) resolveTraceID(parent *Span) string {
    if parent != nil {
        return parent.TraceID
    }
    return generateID(16)
}

func generateID(byteLen int) string {
    b := make([]byte, byteLen)
    rand.Read(b)
    return hex.EncodeToString(b)
}

func SpanFromContext(ctx context.Context) *Span {
    if s, ok := ctx.Value(spanContextKey{}).(*Span); ok {
        return s
    }
    return nil
}
```

### 6.2 SDK — 便捷 API (defer 模式)

```go
// 用户使用方式：
func HandleOrder(ctx context.Context, req *OrderRequest) (*OrderResponse, error) {
    ctx, span := tracer.StartSpan(ctx, "HandleOrder", prism.WithKind(prism.KindServer))
    defer tracer.FinishSpan(span)

    // 自动传播到下游
    user, err := userClient.GetUser(ctx, req.UserID)
    if err != nil {
        span.SetError(err)
        return nil, err
    }
    span.SetTag("user.id", user.ID)

    // ...
    return resp, nil
}
```

### 6.3 SDK — 上下文传播 (跨服务)

```go
package propagation

import "net/http"

const (
    HeaderTraceID      = "X-Prism-Trace-Id"
    HeaderSpanID       = "X-Prism-Span-Id"
    HeaderSampled      = "X-Prism-Sampled"
)

// Inject 向 HTTP headers 注入 trace 上下文
func Inject(span *Span, header http.Header) {
    if span == nil {
        return
    }
    header.Set(HeaderTraceID, span.TraceID)
    header.Set(HeaderSpanID, span.SpanID)
    header.Set(HeaderSampled, "1")
}

// Extract 从 HTTP headers 提取 trace 上下文
func Extract(header http.Header) *SpanContext {
    traceID := header.Get(HeaderTraceID)
    if traceID == "" {
        return nil
    }
    return &SpanContext{
        TraceID:      traceID,
        ParentSpanID: header.Get(HeaderSpanID),
        Sampled:      header.Get(HeaderSampled) == "1",
    }
}

// Kafka / gRPC 同理，只是载体不同
// Kafka: 放在 message headers
// gRPC:  放在 metadata
```

### 6.4 SDK — 自动埋点 (中间件)

```go
// HTTP Server 中间件（自动为每个请求创建 root span）
func HTTPServerMiddleware(tracer *Tracer) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            spanCtx := propagation.Extract(r.Header)
            operation := r.Method + " " + r.URL.Path

            ctx, span := tracer.StartSpan(r.Context(), operation, WithKind(KindServer))
            if spanCtx != nil {
                span.TraceID = spanCtx.TraceID
                span.ParentSpanID = spanCtx.ParentSpanID
            }

            span.SetTag("http.method", r.Method)
            span.SetTag("http.url", r.URL.String())

            // 包装 ResponseWriter 捕获状态码
            ww := &statusWriter{ResponseWriter: w, status: 200}
            next.ServeHTTP(ww, r.WithContext(ctx))

            span.SetTag("http.status_code", strconv.Itoa(ww.status))
            if ww.status >= 400 {
                span.Status = StatusError
            }
            tracer.FinishSpan(span)
        })
    }
}

// HTTP Client 拦截器（自动注入 trace context + 创建 client span）
type TracedTransport struct {
    Tracer    *Tracer
    Wrapped   http.RoundTripper
}

func (t *TracedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    ctx, span := t.Tracer.StartSpan(req.Context(),
        req.Method+" "+req.URL.Host+req.URL.Path,
        WithKind(KindClient),
    )
    defer t.Tracer.FinishSpan(span)

    propagation.Inject(span, req.Header)
    span.SetTag("http.method", req.Method)
    span.SetTag("peer.service", req.URL.Host)

    resp, err := t.Wrapped.RoundTrip(req.WithContext(ctx))
    if err != nil {
        span.SetError(err)
        return nil, err
    }
    span.SetTag("http.status_code", strconv.Itoa(resp.StatusCode))
    return resp, nil
}

// database/sql 拦截器
type TracedDB struct {
    Tracer *Tracer
    DB     *sql.DB
}

func (t *TracedDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
    ctx, span := t.Tracer.StartSpan(ctx, "sql.Query", WithKind(KindClient))
    defer t.Tracer.FinishSpan(span)

    span.SetTag("db.type", "sql")
    span.SetTag("db.statement", truncate(query, 500))

    rows, err := t.DB.QueryContext(ctx, query, args...)
    if err != nil {
        span.SetError(err)
    }
    return rows, err
}
```

### 6.5 SDK — 批量异步上报

```go
package prism

type BatchExporter struct {
    collectorAddr string
    batchSize     int
    flushInterval time.Duration
    buffer        []*Span
    mu            sync.Mutex
    client        pb.CollectorClient  // gRPC client
}

func (e *BatchExporter) Enqueue(span *Span) {
    e.mu.Lock()
    e.buffer = append(e.buffer, span)
    shouldFlush := len(e.buffer) >= e.batchSize
    e.mu.Unlock()

    if shouldFlush {
        go e.Flush()
    }
}

func (e *BatchExporter) Flush() {
    e.mu.Lock()
    if len(e.buffer) == 0 {
        e.mu.Unlock()
        return
    }
    batch := e.buffer
    e.buffer = make([]*Span, 0, e.batchSize)
    e.mu.Unlock()

    // 转换为 protobuf 批量发送
    pbBatch := &pb.SpanBatch{
        Spans: make([]*pb.Span, len(batch)),
    }
    for i, s := range batch {
        pbBatch.Spans[i] = s.ToProto()
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    _, err := e.client.Report(ctx, pbBatch)
    if err != nil {
        // 发送失败：放回 buffer 等下次重试（有上限）
        e.mu.Lock()
        if len(e.buffer)+len(batch) < e.batchSize*10 {
            e.buffer = append(batch, e.buffer...)
        }
        // 超过上限就丢弃，tracing 数据可以丢
        e.mu.Unlock()
    }
}

// 定时刷新 goroutine
func (e *BatchExporter) flushLoop() {
    ticker := time.NewTicker(e.flushInterval)
    for range ticker.C {
        e.Flush()
    }
}
```

### 6.6 采样策略

```go
package prism

type Sampler interface {
    ShouldSample(span *Span) bool
}

// AdaptiveSampler 自适应采样
type AdaptiveSampler struct {
    baseRate   float64  // 基础采样率 0.1 = 10%
    maxPerSec  int64    // 每秒最大 span 数
    counter    atomic.Int64
    lastReset  atomic.Int64
}

func NewAdaptiveSampler(baseRate float64, maxPerSec int64) *AdaptiveSampler {
    s := &AdaptiveSampler{
        baseRate:  baseRate,
        maxPerSec: maxPerSec,
    }
    s.lastReset.Store(time.Now().Unix())
    return s
}

func (s *AdaptiveSampler) ShouldSample(span *Span) bool {
    // 规则 1：错误必采
    if span.Status == StatusError {
        return true
    }

    // 规则 2：慢请求必采 (>1s)
    if span.DurationUs > 1_000_000 {
        return true
    }

    // 规则 3：流量限制
    now := time.Now().Unix()
    if now != s.lastReset.Load() {
        s.counter.Store(0)
        s.lastReset.Store(now)
    }
    if s.counter.Load() >= s.maxPerSec {
        return false
    }

    // 规则 4：基于 trace_id hash 的概率采样
    // 同一个 trace 的所有 span 采样结果一致
    h := fnv32(span.TraceID)
    sampled := float64(h%10000) < s.baseRate*10000
    if sampled {
        s.counter.Add(1)
    }
    return sampled
}

func fnv32(s string) uint32 {
    h := uint32(2166136261)
    for i := 0; i < len(s); i++ {
        h *= 16777619
        h ^= uint32(s[i])
    }
    return h
}
```

### 6.7 Collector — 接收 + 写入

```go
package collector

type Collector struct {
    buffer     []*pb.Span
    mu         sync.Mutex
    flushSize  int
    flushInterval time.Duration
    clickhouse *ClickHouseWriter
    depTracker *DependencyTracker
}

// Report 实现 gRPC 接口，接收 SDK 上报
func (c *Collector) Report(ctx context.Context, batch *pb.SpanBatch) (*pb.ReportResponse, error) {
    c.mu.Lock()
    for _, span := range batch.Spans {
        // 补充衍生数据
        c.depTracker.Record(span)
        c.buffer = append(c.buffer, span)
    }
    shouldFlush := len(c.buffer) >= c.flushSize
    c.mu.Unlock()

    if shouldFlush {
        go c.flush()
    }

    return &pb.ReportResponse{Accepted: int32(len(batch.Spans))}, nil
}

func (c *Collector) flush() {
    c.mu.Lock()
    if len(c.buffer) == 0 {
        c.mu.Unlock()
        return
    }
    batch := c.buffer
    c.buffer = make([]*pb.Span, 0, c.flushSize)
    c.mu.Unlock()

    if err := c.clickhouse.BatchInsert(batch); err != nil {
        slog.Error("flush to clickhouse failed", "error", err, "count", len(batch))
        // 写失败可以丢弃，tracing 数据不是关键路径
    }
}

// DependencyTracker 从 span 中提取服务依赖关系
type DependencyTracker struct {
    redis *redis.Client
}

func (d *DependencyTracker) Record(span *pb.Span) {
    if span.Kind == pb.SpanKind_CLIENT && span.ParentSpanId != "" {
        peerService := ""
        for _, tag := range span.Tags {
            if tag.Key == "peer.service" {
                peerService = tag.Value
                break
            }
        }
        if peerService != "" {
            key := fmt.Sprintf("prism:dep:%s:%s", span.Service, peerService)
            d.redis.Incr(context.Background(), key)
            d.redis.Expire(context.Background(), key, 24*time.Hour)
        }
    }
}
```

## 七、API 设计

### 7.1 Trace 查询

```
GET /api/v1/traces/:traceID
    → 返回完整的 trace（所有 span，组装成树形结构）

GET /api/v1/traces?service=order-svc&operation=POST+/api/orders&min_duration=1000ms&status=error&start=2024-01-01T00:00:00Z&end=2024-01-02T00:00:00Z&limit=20
    → 按条件搜索 trace
```

### 7.2 服务信息

```
GET /api/v1/services
    → 返回所有服务列表 + 最近 1h 的 QPS、错误率、P99 延迟

GET /api/v1/services/:name/operations
    → 返回某服务的所有 operation + 各自的统计

GET /api/v1/dependencies?start=...&end=...
    → 返回服务依赖拓扑 {nodes: [...], edges: [...]}
```

### 7.3 统计

```
GET /api/v1/stats/latency?service=order-svc&operation=POST+/api/orders&start=...&end=...&granularity=1m
    → 返回延迟趋势: [{timestamp, p50, p90, p99, max}]

GET /api/v1/stats/throughput?service=order-svc&start=...&end=...&granularity=1m
    → 返回吞吐趋势: [{timestamp, total, errors, error_rate}]
```

## 八、用户使用示例

```go
package main

import (
    "net/http"
    "github.com/yourname/prism"
)

func main() {
    // 初始化 Tracer
    tracer := prism.NewTracer("order-service",
        prism.WithCollector("prism-collector:4317"),
        prism.WithSampler(prism.NewAdaptiveSampler(0.1, 1000)),
    )
    defer tracer.Shutdown()

    // 自动埋点：HTTP Server
    mux := http.NewServeMux()
    mux.HandleFunc("/api/orders", createOrder)
    handler := prism.HTTPServerMiddleware(tracer)(mux)

    // 自动埋点：HTTP Client
    httpClient := &http.Client{
        Transport: &prism.TracedTransport{
            Tracer:  tracer,
            Wrapped: http.DefaultTransport,
        },
    }

    // 自动埋点：database/sql
    db := prism.WrapDB(tracer, rawDB)

    http.ListenAndServe(":8080", handler)
}

func createOrder(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // 手动创建子 span（可选）
    ctx, span := tracer.StartSpan(ctx, "validateOrder")
    // ... 校验逻辑
    tracer.FinishSpan(span)

    // 调用其他服务（自动传播 trace context）
    req, _ := http.NewRequestWithContext(ctx, "GET", "http://user-svc/api/users/123", nil)
    httpClient.Do(req)  // 自动注入 X-Prism-Trace-Id header

    // 查询数据库（自动记录 SQL span）
    db.QueryContext(ctx, "SELECT * FROM orders WHERE id = $1", orderID)
}
```

## 九、项目结构

```
prism/
├── cmd/
│   ├── collector/
│   │   └── main.go              # Collector 服务入口
│   └── query/
│       └── main.go              # Query 服务入口
├── sdk/                         # SDK (用户 import 的包)
│   ├── tracer.go                # Tracer 核心
│   ├── span.go                  # Span 定义
│   ├── sampler.go               # 采样策略
│   ├── exporter.go              # 批量上报
│   ├── propagation/
│   │   ├── http.go              # HTTP header 注入/提取
│   │   ├── grpc.go              # gRPC metadata
│   │   └── kafka.go             # Kafka headers
│   └── middleware/
│       ├── http_server.go       # HTTP Server 中间件
│       ├── http_client.go       # HTTP Client 拦截
│       ├── grpc_server.go       # gRPC Server 拦截器
│       ├── grpc_client.go       # gRPC Client 拦截器
│       └── sql.go               # database/sql 包装
├── internal/
│   ├── collector/
│   │   ├── collector.go         # 接收 + 攒批
│   │   ├── writer.go            # ClickHouse 写入
│   │   └── dependency.go        # 依赖关系提取
│   ├── query/
│   │   ├── server.go            # HTTP API
│   │   ├── handler_trace.go     # Trace 查询
│   │   ├── handler_service.go   # 服务信息
│   │   ├── handler_stats.go     # 统计查询
│   │   └── handler_deps.go      # 依赖拓扑
│   └── storage/
│       ├── storage.go           # 接口定义
│       └── clickhouse.go        # ClickHouse 实现
├── proto/
│   ├── span.proto               # Span 定义
│   └── collector.proto          # Collector gRPC 服务
├── deploy/
│   ├── docker-compose.yml       # 一键启动（Collector + ClickHouse + Redis）
│   └── clickhouse-init.sql      # 建表语句
├── examples/
│   └── microservices/           # 多服务示例
├── go.mod
└── Makefile
```

## 十、关键 Metrics

| Metric | 类型 | 说明 |
|--------|------|------|
| `prism_collector_spans_received_total` | Counter | Collector 收到的 span 总数 |
| `prism_collector_spans_dropped_total` | Counter | 被采样丢弃的 span 数 |
| `prism_collector_flush_duration_seconds` | Histogram | 写入 ClickHouse 耗时 |
| `prism_collector_buffer_size` | Gauge | 当前缓冲区大小 |
| `prism_sdk_export_errors_total` | Counter | SDK 上报失败次数 |
| `prism_query_latency_seconds` | Histogram | 查询 API 延迟 |

## 十一、容量估算

假设：10 个服务，每个服务 1000 QPS，每个请求平均 5 个 span，采样率 10%

```
原始 span 量:  10 × 1000 × 5 = 50,000 spans/s
采样后:        50,000 × 10% = 5,000 spans/s
每个 span ~300 bytes (压缩后)
存储:          5,000 × 300 × 86400 = ~130 GB/day
14 天保留:     ~1.8 TB
```

ClickHouse 压缩比通常 5-10x，实际存储 ~200-400 GB。
