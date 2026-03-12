# Prism 技术设计文档

> 一束请求穿过多个服务，Prism 把它折射成清晰可见的完整链路

## 一、技术栈

| 组件 | 选型 | 理由 |
|------|------|------|
| 语言 | **Go 1.22+** | Jaeger/Zipkin Collector 都是 Go，高吞吐低延迟 |
| SDK | **Go SDK** | 基于 `context.Context` 传播，零分配优化 |
| 采集协议 | **Protobuf over gRPC** | 比 JSON 小 5-10x，SDK → Collector 高效传输 |
| Collector | **自研 Go 服务** | 接收、采样、攒批、写入 |
| 存储 | **ClickHouse** | 列式存储，时序查询极快，自带 TTL 过期，压缩比高 |
| 索引辅助 | **ClickHouse 物化视图** | 按 service/operation 预聚合 |
| 依赖图缓存 | **Redis** | 服务调用关系快速读取 |
| Query API | **Go + chi** | 给前端/告警系统查询用 |

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
```

每个 Span = 一次操作的时间切片：

| 字段 | 说明 |
|------|------|
| `trace_id` | 贯穿所有服务，唯一标识这次请求（128-bit） |
| `span_id` | 当前操作的唯一 ID（64-bit） |
| `parent_span_id` | 谁触发了我 |
| `operation` | 操作名称，如 `POST /api/orders` |
| `service` | 所属服务名 |
| `kind` | INTERNAL / SERVER / CLIENT / PRODUCER / CONSUMER |
| `start_us` | 开始时间（微秒时间戳） |
| `duration_us` | 耗时（微秒） |
| `status` | OK / ERROR |
| `tags` | 键值对，如 `http.method=POST` |
| `events` | 时间点事件，如异常、重试 |

## 三、整体架构

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
│                  Collector Cluster (无状态，可水平扩展)                │
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
                              │  │  spans 表              │      │
                              │  │  ORDER BY (trace_id)  │      │
                              │  └───────────────────────┘      │
                              │  ┌───────────────────────┐      │
                              │  │  service_operations    │      │
                              │  │  (Materialized View)  │      │
                              │  └───────────────────────┘      │
                              │  ┌───────────────────────┐      │
                              │  │  service_dependencies  │      │
                              │  │  (Materialized View)  │      │
                              │  └───────────────────────┘      │
                              └────────────────┬────────────────┘
                                               │
                              ┌────────────────▼────────────────┐
                              │        Query Service            │
                              │                                 │
                              │  GET /api/v1/traces/:id         │
                              │  GET /api/v1/traces             │
                              │  GET /api/v1/services           │
                              │  GET /api/v1/services/:name/ops │
                              │  GET /api/v1/dependencies       │
                              │  GET /api/v1/stats              │
                              └─────────────────────────────────┘
```

## 四、数据模型

### 4.1 Protobuf 定义

```protobuf
message Span {
    bytes  trace_id       = 1;  // 16 bytes (128-bit)
    bytes  span_id        = 2;  // 8 bytes (64-bit)
    bytes  parent_span_id = 3;  // 8 bytes, root span 为空
    string operation      = 4;  // "POST /api/orders"
    string service        = 5;  // "order-svc"
    SpanKind kind         = 6;
    uint64 start_us       = 7;  // 微秒时间戳
    uint64 duration_us    = 8;
    StatusCode status     = 9;
    repeated KeyValue tags   = 10;
    repeated SpanEvent events = 11;
}

message SpanBatch {
    repeated Span spans = 1;  // SDK 批量上报
}
```

### 4.2 ClickHouse 建表

```sql
-- 主表
CREATE TABLE spans (
    trace_id         FixedString(32),
    span_id          FixedString(16),
    parent_span_id   String DEFAULT '',
    operation        LowCardinality(String),
    service          LowCardinality(String),
    kind             LowCardinality(String),
    start_us         UInt64,
    duration_us      UInt64,
    status           LowCardinality(String),
    tags             Map(String, String),
    events           String,
    date             Date DEFAULT toDate(fromUnixTimestamp64Micro(start_us))
)
ENGINE = MergeTree()
PARTITION BY date
ORDER BY (trace_id, start_us)
TTL date + INTERVAL 14 DAY;
```

**二级索引**：`service` 使用 set 索引，`operation` 使用 tokenbf 索引。

**物化视图**：
- `service_operations_mv` — 按 service + operation + date 聚合调用次数、错误数、延迟
- `service_dependencies_mv` — 从 span 父子关系中提取服务间调用关系

## 五、核心模块设计

### 5.1 SDK — Tracer

入口类，管理 Span 生命周期：

```go
tracer := prism.NewTracer("order-service",
    prism.WithCollector("localhost:24317"),
    prism.WithSampler(prism.NewAdaptiveSampler(0.1, 1000)),
)
defer tracer.Shutdown()

// 使用 defer 模式：
ctx, span := tracer.StartSpan(ctx, "HandleOrder", prism.WithKind(prism.KindServer))
defer tracer.FinishSpan(span)
```

核心流程：`StartSpan` 创建 Span 并注入 context → 业务逻辑执行 → `FinishSpan` 计算耗时并提交给采样器 → 采样通过则入队 BatchExporter。

### 5.2 SDK — 上下文传播

跨服务通过 HTTP Header / gRPC Metadata / Kafka Header 传播 trace context：

| 载体 | Header |
|------|--------|
| HTTP | `X-Prism-Trace-Id`, `X-Prism-Span-Id`, `X-Prism-Sampled` |
| gRPC | `x-prism-trace-id`, `x-prism-span-id`, `x-prism-sampled` |
| Kafka | 同上，放在 message headers |

### 5.3 SDK — 自动埋点中间件

| 中间件 | 作用 |
|--------|------|
| `HTTPServerMiddleware` | 自动为每个入站请求创建 server span，记录 method/url/status_code |
| `TracedTransport` | http.RoundTripper 包装，自动创建 client span + 注入 trace header |
| `UnaryServerInterceptor` | gRPC server 拦截器，自动创建 server span |
| `UnaryClientInterceptor` | gRPC client 拦截器，自动创建 client span + 注入 metadata |
| `TracedDB` | database/sql 包装，自动记录 SQL query span |

### 5.4 SDK — 批量异步上报

```
Span → Enqueue → Buffer (内存)
                    │
         ┌──────────┴──────────┐
         │ 满 batchSize 条    │ 或 │ 每 flushInterval │
         └──────────┬──────────┘
                    ▼
         gRPC Report(SpanBatch) → Collector
                    │
               失败 → 放回 buffer（上限 10x batchSize，超过丢弃）
```

### 5.5 采样策略

`AdaptiveSampler` 四层规则：

| 优先级 | 规则 | 说明 |
|--------|------|------|
| 1 | 错误必采 | `status == ERROR` 直接采样 |
| 2 | 慢请求必采 | `duration > 1s` 直接采样 |
| 3 | 流量限制 | 超过 `maxPerSec` 直接丢弃 |
| 4 | 概率采样 | 基于 `trace_id` 的 FNV hash，同一 trace 所有 span 采样结果一致 |

### 5.6 Collector

接收 SDK 上报的 SpanBatch，攒批写入 ClickHouse：

- gRPC 接口：`Report(SpanBatch) → ReportResponse`
- 缓冲区：5000 条或 5 秒触发一次 flush
- 写入失败可丢弃（tracing 数据非关键路径）
- DependencyTracker：从 CLIENT span 提取 `peer.service` tag，写入 Redis 记录服务间调用关系

### 5.7 Query Service

基于 chi 的 HTTP API，查询 ClickHouse：

- Trace 查询：按 trace_id 精确查找，或按 service/operation/status/duration/时间范围搜索
- 服务信息：从物化视图读取 QPS、错误率、P99 延迟
- 依赖拓扑：返回 `{nodes: [...], edges: [...]}` 结构
- 统计时序：按指定粒度返回延迟分位数和吞吐量

## 六、部署架构

```
最小部署（单机）:
  1 个 Collector 进程（gRPC :24317）
  1 个 Query 进程（HTTP :28080）
  1 个 ClickHouse
  1 个 Redis

水平扩展:
  N 个 Collector 实例（无状态，负载均衡器分发 gRPC）
  N 个 Query 实例（无状态，负载均衡器分发 HTTP）
  ClickHouse 集群（ReplicatedMergeTree）
  Redis Sentinel / Cluster
```

## 七、关键 Metrics

| Metric | 类型 | 说明 |
|--------|------|------|
| `prism_collector_spans_received_total` | Counter | Collector 收到的 span 总数 |
| `prism_collector_spans_dropped_total` | Counter | 被采样丢弃的 span 数 |
| `prism_collector_flush_duration_seconds` | Histogram | 写入 ClickHouse 耗时 |
| `prism_collector_buffer_size` | Gauge | 当前缓冲区大小 |
| `prism_sdk_export_errors_total` | Counter | SDK 上报失败次数 |
| `prism_query_latency_seconds` | Histogram | 查询 API 延迟 |

## 八、容量估算

假设：10 个服务，每个服务 1000 QPS，每个请求平均 5 个 span，采样率 10%

```
原始 span 量:  10 × 1000 × 5 = 50,000 spans/s
采样后:        50,000 × 10% = 5,000 spans/s
每个 span ~300 bytes (压缩后)
存储:          5,000 × 300 × 86400 = ~130 GB/day
14 天保留:     ~1.8 TB
```

ClickHouse 压缩比通常 5-10x，实际存储 ~200-400 GB。

## 九、安全考虑

1. **gRPC 传输**：生产环境应启用 TLS（SDK → Collector）
2. **Query API**：建议在前面加认证网关，不要直接暴露公网
3. **ClickHouse 访问**：使用独立用户和密码，限制网络访问
4. **数据脱敏**：SDK 默认截断 SQL 语句（500 字符），避免记录敏感数据
5. **TTL 过期**：数据 14 天自动清理，无需手动维护
6. **日志脱敏**：不在日志中打印完整的 trace 原始数据
