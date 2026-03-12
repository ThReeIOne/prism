# Prism — Distributed Tracing / 分布式链路追踪

> A request passes through multiple services — Prism refracts it into a clear, visible trace.
>
> 一束请求穿过多个服务，Prism 把它折射成清晰可见的完整链路。

[English](#features) | [中文](#功能特性)

---

## Features

- **Automatic instrumentation** — HTTP server/client, gRPC, database/sql middleware out of the box
- **Context propagation** — Seamless trace context across HTTP, gRPC, and Kafka boundaries
- **Adaptive sampling** — Errors and slow requests always captured, hash-based consistent sampling per trace
- **Batch async export** — SDK buffers spans and sends via gRPC in configurable batches
- **ClickHouse storage** — Column-oriented storage with automatic TTL, materialized views for aggregation
- **Query API** — Search traces, service topology, latency/throughput statistics via REST
- **Lightweight SDK** — Based on `context.Context`, zero-config defaults, `defer` pattern for span lifecycle

## Quick Start

### Docker Compose (recommended)

```bash
# Start ClickHouse + Redis + Collector + Query
cd deploy && docker compose up -d

# Verify services are running
curl http://localhost:28080/health
```

### From Source

```bash
# Build
make build

# Run Collector (requires ClickHouse + Redis)
./bin/prism-collector -listen=:24317 -clickhouse=localhost:29000 -redis=localhost:26379

# Run Query Server
./bin/prism-query -listen=:28080 -clickhouse=localhost:29000
```

### Try It Out

```bash
# Run the example microservices demo
go run ./examples/microservices/

# Send a request through the call chain
curl http://localhost:8081/api/orders/123

# Search traces via Query API
curl http://localhost:28080/api/v1/traces | jq .

# Get service list with stats
curl http://localhost:28080/api/v1/services | jq .

# View service dependency topology
curl http://localhost:28080/api/v1/dependencies | jq .
```

### SDK Integration

```go
// Initialize tracer
tracer := prism.NewTracer("order-service",
    prism.WithCollector("localhost:24317"),
    prism.WithSampler(prism.NewAdaptiveSampler(0.1, 1000)),
)
defer tracer.Shutdown()

// Auto-instrument HTTP server
handler := prism.HTTPServerMiddleware(tracer)(mux)

// Auto-instrument HTTP client
httpClient := &http.Client{
    Transport: &prism.TracedTransport{Tracer: tracer, Wrapped: http.DefaultTransport},
}
```

## Configuration

### Collector Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-listen` | `:24317` | gRPC listen address |
| `-clickhouse` | `localhost:29000` | ClickHouse native address |
| `-ch-db` | `prism` | ClickHouse database name |
| `-ch-user` | `default` | ClickHouse username |
| `-ch-pass` | (empty) | ClickHouse password |
| `-redis` | `localhost:26379` | Redis address |
| `-flush-size` | `5000` | Batch flush threshold |

### Query Server Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-listen` | `:28080` | HTTP listen address |
| `-clickhouse` | `localhost:29000` | ClickHouse native address |
| `-ch-db` | `prism` | ClickHouse database name |

### SDK Options

| Option | Default | Description |
|--------|---------|-------------|
| `WithCollector(addr)` | `localhost:24317` | Collector gRPC address |
| `WithBatchSize(n)` | `1024` | Spans per batch |
| `WithFlushInterval(d)` | `5s` | Batch flush interval |
| `WithSampler(s)` | `AlwaysSampler` | Sampling strategy |

## API Overview

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/traces/{traceID}` | Get complete trace |
| `GET` | `/api/v1/traces?service=...&status=...` | Search traces with filters |
| `GET` | `/api/v1/services` | List services with QPS, error rate, P99 |
| `GET` | `/api/v1/services/{name}/operations` | List operations for a service |
| `GET` | `/api/v1/dependencies?start=...&end=...` | Service dependency topology |
| `GET` | `/api/v1/stats/latency?service=...&granularity=1m` | Latency time series (P50/P90/P99) |
| `GET` | `/api/v1/stats/throughput?service=...&granularity=1m` | Throughput time series |
| `GET` | `/health` | Health check |

## Development

```bash
make build        # Build collector + query binaries
make test         # Run tests
make lint         # Run linter
make proto        # Regenerate protobuf code
make docker-up    # Start docker-compose stack
make docker-down  # Stop docker-compose stack
make deps         # Run go mod tidy
```

## Project Structure

```
prism/
├── cmd/collector/        # Collector gRPC service entry point
├── cmd/query/            # Query HTTP service entry point
├── sdk/                  # SDK (user-facing package)
│   ├── propagation/      # Context propagation (HTTP, gRPC, Kafka)
│   └── middleware/       # Auto-instrumentation middleware
├── internal/collector/   # Collector: receive, batch, write
├── internal/query/       # Query API handlers
├── internal/storage/     # Storage interface + ClickHouse impl
├── proto/                # Protobuf definitions
├── deploy/               # Docker Compose, Dockerfiles, init SQL
└── examples/             # Multi-service demo
```

## Architecture

See [docs/design.md](docs/design.md) for the full technical design document.

## License

MIT

---

# 中文文档

## 功能特性

- **自动埋点** — HTTP Server/Client、gRPC、database/sql 中间件开箱即用
- **上下文传播** — HTTP Header、gRPC Metadata、Kafka Header 无缝传递 trace 上下文
- **自适应采样** — 错误和慢请求必采，基于 trace_id hash 的一致性概率采样
- **批量异步上报** — SDK 内存缓冲，gRPC 批量发送，可配置 batch 大小和刷新间隔
- **ClickHouse 存储** — 列式存储，自带 TTL 过期，物化视图预聚合
- **查询 API** — 链路搜索、服务拓扑、延迟/吞吐统计，RESTful 接口
- **轻量 SDK** — 基于 `context.Context`，零配置默认值，`defer` 模式管理 Span 生命周期

## 快速开始

### Docker Compose（推荐）

```bash
# 启动 ClickHouse + Redis + Collector + Query
cd deploy && docker compose up -d

# 验证服务运行
curl http://localhost:28080/health
```

### 从源码构建

```bash
# 编译
make build

# 运行 Collector（需要 ClickHouse + Redis）
./bin/prism-collector -listen=:24317 -clickhouse=localhost:29000 -redis=localhost:26379

# 运行 Query 服务
./bin/prism-query -listen=:28080 -clickhouse=localhost:29000
```

### 试一试

```bash
# 运行多服务示例
go run ./examples/microservices/

# 发送请求，经过完整调用链
curl http://localhost:8081/api/orders/123

# 查询 trace
curl http://localhost:28080/api/v1/traces | jq .

# 查看服务列表（含 QPS、错误率、P99）
curl http://localhost:28080/api/v1/services | jq .

# 查看服务依赖拓扑图
curl http://localhost:28080/api/v1/dependencies | jq .
```

### SDK 接入

```go
// 初始化 Tracer
tracer := prism.NewTracer("order-service",
    prism.WithCollector("localhost:24317"),
    prism.WithSampler(prism.NewAdaptiveSampler(0.1, 1000)),
)
defer tracer.Shutdown()

// HTTP Server 自动埋点
handler := prism.HTTPServerMiddleware(tracer)(mux)

// HTTP Client 自动埋点
httpClient := &http.Client{
    Transport: &prism.TracedTransport{Tracer: tracer, Wrapped: http.DefaultTransport},
}
```

## 配置说明

### Collector 参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-listen` | `:24317` | gRPC 监听地址 |
| `-clickhouse` | `localhost:29000` | ClickHouse 原生协议地址 |
| `-ch-db` | `prism` | ClickHouse 数据库名 |
| `-ch-user` | `default` | ClickHouse 用户名 |
| `-ch-pass` | (空) | ClickHouse 密码 |
| `-redis` | `localhost:26379` | Redis 地址 |
| `-flush-size` | `5000` | 批量刷入阈值 |

### Query 服务参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-listen` | `:28080` | HTTP 监听地址 |
| `-clickhouse` | `localhost:29000` | ClickHouse 原生协议地址 |
| `-ch-db` | `prism` | ClickHouse 数据库名 |

### SDK 配置项

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithCollector(addr)` | `localhost:24317` | Collector gRPC 地址 |
| `WithBatchSize(n)` | `1024` | 每批 Span 数量 |
| `WithFlushInterval(d)` | `5s` | 批量刷新间隔 |
| `WithSampler(s)` | `AlwaysSampler` | 采样策略 |

## API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/traces/{traceID}` | 获取完整 trace（所有 span） |
| `GET` | `/api/v1/traces?service=...&status=...` | 按条件搜索 trace |
| `GET` | `/api/v1/services` | 服务列表（含 QPS、错误率、P99 延迟） |
| `GET` | `/api/v1/services/{name}/operations` | 某服务的所有操作及统计 |
| `GET` | `/api/v1/dependencies?start=...&end=...` | 服务依赖拓扑（节点 + 边） |
| `GET` | `/api/v1/stats/latency?service=...&granularity=1m` | 延迟趋势（P50/P90/P99） |
| `GET` | `/api/v1/stats/throughput?service=...&granularity=1m` | 吞吐趋势（总量/错误数） |
| `GET` | `/health` | 健康检查 |

## 开发命令

```bash
make build        # 编译 collector + query 二进制
make test         # 运行测试
make lint         # 代码检查
make proto        # 重新生成 protobuf 代码
make docker-up    # 启动 docker-compose
make docker-down  # 停止 docker-compose
make deps         # 整理依赖 (go mod tidy)
```

## 项目结构

```
prism/
├── cmd/collector/        # Collector gRPC 服务入口
├── cmd/query/            # Query HTTP 服务入口
├── sdk/                  # SDK（用户引入的包）
│   ├── propagation/      # 上下文传播（HTTP、gRPC、Kafka）
│   └── middleware/       # 自动埋点中间件
├── internal/collector/   # Collector：接收、攒批、写入
├── internal/query/       # Query API 处理器
├── internal/storage/     # 存储接口 + ClickHouse 实现
├── proto/                # Protobuf 定义
├── deploy/               # Docker Compose、Dockerfile、建表 SQL
└── examples/             # 多服务示例
```

## 详细文档

- [技术设计文档](docs/design.md) — 架构设计、数据模型、模块详解、采样策略、容量估算
- [使用指南](docs/guide.md) — 落地场景、SDK 接入速查、采样配置、部署建议
