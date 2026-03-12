# Prism 使用指南

## 一、Prism 是什么

Prism 是一个**分布式链路追踪系统**，自动采集请求在多个服务间的调用链路、耗时、错误信息。

简单来说：**一个请求进来，经过了哪些服务、每步花了多久、哪里出了错，Prism 帮你看得清清楚楚。**

```
用户请求 ──→ API Gateway ──→ Order Service ──→ User Service
                                           ──→ Redis
                                           ──→ Kafka
                  ↓
            Prism 自动记录整条链路
                  ↓
            查询界面看到完整的瀑布图
```

## 二、解决什么问题

### 没有 Prism 时

```
用户反馈："下单接口好慢"
你："哪里慢？"  →  开始翻日志
  → grep 订单服务日志，没发现异常
  → 跑到用户服务看日志，发现有个慢查询
  → 但不确定是不是同一个请求
  → 人肉拼凑时间戳……
```

常见痛点：
- 一个请求经过 API → DB → Redis → Kafka → 下游服务，某环节慢了或出错了，定位困难
- 日志散落在多个服务，排查问题需要跨机器 grep，靠时间戳人肉拼凑
- 不知道服务间的真实调用关系和依赖
- 性能优化没有数据支撑，不知道瓶颈在哪

### 有了 Prism 后

```
用户反馈："下单接口好慢"
你：打开 Prism 搜索该请求的 trace
  → 一眼看到完整调用链
  → Order Service 的 SQL 查询花了 800ms
  → 问题定位完毕
```

- 一个 trace ID 串联所有服务的所有操作
- 瀑布图展示每一步的耗时，瓶颈一目了然
- 服务依赖拓扑自动生成，不用手画架构图
- 错误和慢请求自动采样，问题自动上报

## 三、核心概念

| 概念 | 说明 | 举例 |
|------|------|------|
| **Trace** | 一次完整请求的生命周期 | 用户下单的整个过程 |
| **Span** | 一次具体操作的时间切片 | 一次 HTTP 调用、一次 SQL 查询 |
| **trace_id** | 贯穿所有服务的唯一标识 | `a1b2c3d4e5f6...`（128-bit） |
| **parent_span_id** | 谁触发了这个操作 | 建立 span 之间的父子关系 |
| **Tags** | 键值对元数据 | `http.method=POST`, `db.statement=SELECT...` |
| **Events** | 时间点事件 | 异常、重试、状态变更 |

## 四、日常项目落地场景

### 场景 1：微服务接口慢查询定位

线上用户反馈某接口偶尔很慢，但你不知道慢在哪个环节。

```
┌──────────┐     ┌───────────┐     ┌──────────┐
│ API GW   │────→│ Order Svc │────→│ User Svc │
└──────────┘     │           │────→│ Redis    │
                 │           │────→│ Kafka    │
                 └───────────┘     └──────────┘
```

**使用方式：**

1. 各服务接入 SDK（3 行代码）
```go
tracer := prism.NewTracer("order-service",
    prism.WithCollector("prism-collector:24317"),
)
defer tracer.Shutdown()
handler := prism.HTTPServerMiddleware(tracer)(mux)
```

2. 搜索慢请求
```bash
curl "http://localhost:28080/api/v1/traces?service=order-svc&min_duration=1000ms&limit=10" | jq .
```

3. 查看完整 trace，定位瓶颈 span

**好处：**
- 不用跨机器翻日志，一个 trace ID 看全链路
- 慢请求自动采样（>1s），不用提前开 debug
- 可以按 service/operation/duration/status 组合搜索

### 场景 2：服务依赖治理

微服务越来越多，谁调谁已经说不清了，想做依赖治理。

```
curl "http://localhost:28080/api/v1/dependencies" | jq .
```

返回：
```json
{
  "nodes": [
    {"id": "api-gateway", "label": "api-gateway"},
    {"id": "order-svc", "label": "order-svc"},
    {"id": "user-svc", "label": "user-svc"}
  ],
  "edges": [
    {"source": "api-gateway", "target": "order-svc", "call_count": 15000},
    {"source": "order-svc", "target": "user-svc", "call_count": 15000}
  ]
}
```

**好处：**
- 基于真实流量自动生成，不靠人维护
- 可以发现意外的调用关系（"这个服务怎么还调了那个？"）
- 包含调用量和错误率，可以量化依赖健康度

### 场景 3：上线前后性能对比

新版本上线，想看延迟有没有变化。

```bash
# 上线前的 P99 延迟
curl "http://localhost:28080/api/v1/stats/latency?service=order-svc&operation=POST+/api/orders&start=2024-01-01T00:00:00Z&end=2024-01-01T12:00:00Z&granularity=5m" | jq .

# 上线后
curl "http://localhost:28080/api/v1/stats/latency?service=order-svc&operation=POST+/api/orders&start=2024-01-01T12:00:00Z&end=2024-01-02T00:00:00Z&granularity=5m" | jq .
```

返回的 P50/P90/P99/Max 时序数据可以直接画图对比。

### 场景 4：错误定位

线上出现间歇性错误，但日志里只有最终错误，不知道根因。

```bash
# 搜索错误 trace
curl "http://localhost:28080/api/v1/traces?service=order-svc&status=error&limit=5" | jq .
```

Prism 的自适应采样器**错误必采**，不用担心采样丢掉了错误 trace。查看完整 trace 可以看到：
- 哪个 span 出了错
- 错误消息和堆栈（如果 SetError 了）
- 错误发生前后的其他操作时序

### 场景 5：容量规划

想知道某个服务的吞吐趋势，评估是否需要扩容。

```bash
curl "http://localhost:28080/api/v1/stats/throughput?service=order-svc&start=2024-01-01T00:00:00Z&end=2024-01-08T00:00:00Z&granularity=1h" | jq .
```

返回每小时的 total/errors/error_rate 时序数据。

## 五、SDK 接入速查

### HTTP Server 自动埋点

```go
tracer := prism.NewTracer("my-service", prism.WithCollector("prism-collector:24317"))
defer tracer.Shutdown()

mux := http.NewServeMux()
mux.HandleFunc("/api/orders", handleOrder)
handler := prism.HTTPServerMiddleware(tracer)(mux)
http.ListenAndServe(":8080", handler)
```

### HTTP Client 自动埋点

```go
httpClient := &http.Client{
    Transport: &prism.TracedTransport{
        Tracer:  tracer,
        Wrapped: http.DefaultTransport,
    },
}

// 自动注入 X-Prism-Trace-Id header
req, _ := http.NewRequestWithContext(ctx, "GET", "http://user-svc/api/users/123", nil)
httpClient.Do(req)
```

### gRPC 自动埋点

```go
// Server
grpcServer := grpc.NewServer(
    grpc.UnaryInterceptor(prism.UnaryServerInterceptor(tracer)),
)

// Client
conn, _ := grpc.Dial("target:50051",
    grpc.WithUnaryInterceptor(prism.UnaryClientInterceptor(tracer)),
)
```

### database/sql 自动埋点

```go
db := prism.WrapDB(tracer, rawDB)
db.QueryContext(ctx, "SELECT * FROM orders WHERE id = $1", orderID)
// 自动记录 sql.Query span + db.statement tag
```

### 手动创建子 Span

```go
ctx, span := tracer.StartSpan(ctx, "validateOrder")
defer tracer.FinishSpan(span)

span.SetTag("order.id", orderID)
if err != nil {
    span.SetError(err)
}
```

## 六、采样策略说明

| 策略 | 说明 | 适用场景 |
|------|------|----------|
| `AlwaysSampler` | 100% 采样 | 开发测试 |
| `NeverSampler` | 0% 采样 | 禁用 tracing |
| `AdaptiveSampler` | 自适应 | **生产推荐** |

`AdaptiveSampler(baseRate, maxPerSec)` 的行为：

| 优先级 | 规则 | 效果 |
|--------|------|------|
| 最高 | 错误必采 | 不管采样率，error 一定记录 |
| 高 | 慢请求必采（>1s） | 性能问题不会被采样丢掉 |
| 中 | 流量限制 | 超过 maxPerSec 直接丢弃，保护 Collector |
| 低 | 概率采样 | 基于 trace_id hash，同一 trace 结果一致 |

推荐配置：`NewAdaptiveSampler(0.1, 1000)` — 10% 基础采样率，每秒最多 1000 个 span。

## 七、部署建议

### 开发环境

```bash
# 一键启动 ClickHouse + Redis + Collector + Query
cd deploy && docker compose up -d

# 本地运行示例
go run ./examples/microservices/
```

### 生产环境

- Collector 是无状态的，可以多实例部署 + gRPC 负载均衡
- Query 是无状态的，可以多实例部署 + HTTP 负载均衡
- ClickHouse 使用 ReplicatedMergeTree 做高可用
- Redis Sentinel 或 Cluster 做高可用

```
LB (gRPC)                         LB (HTTP)
  └──→ Collector x N                └──→ Query x N
         └──→ ClickHouse Cluster           └──→ ClickHouse Cluster
         └──→ Redis Sentinel
```

### 端口说明

| 服务 | 端口 | 说明 |
|------|------|------|
| Collector | 24317 | gRPC，SDK 上报 span |
| Query | 28080 | HTTP，查询 API |
| ClickHouse HTTP | 29123 | ClickHouse HTTP 接口 |
| ClickHouse Native | 29000 | ClickHouse 原生协议 |
| Redis | 26379 | 依赖关系缓存 |

## 八、总结

**什么时候该用 Prism：**
- 微服务架构，请求跨多个服务
- 线上排查慢接口、间歇性错误
- 想了解服务间的真实调用关系
- 性能优化需要数据支撑
- 上线前后需要对比延迟变化

**什么时候不需要：**
- 单体应用，没有跨服务调用（用 pprof 就够了）
- 只需要 Metrics 聚合（用 Prometheus）
- 只需要日志搜索（用 ELK/Loki）
- 已经在用 Jaeger/Zipkin/SkyWalking 并且满足需求
