<div align="center">

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![ClickHouse](https://img.shields.io/badge/ClickHouse-24.1-FFCC01?style=flat-square&logo=clickhouse&logoColor=black)](https://clickhouse.com)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react&logoColor=black)](https://react.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow?style=flat-square)](LICENSE)

# 🔭 Prism

**Distributed Tracing System / 分布式链路追踪系统**

*A request passes through multiple services — Prism refracts it into a clear, visible trace.*

*一束请求穿过多个服务，Prism 把它折射成清晰可见的完整链路。*

[Features](#features) · [Quick Start](#quick-start) · [SDK Integration](#sdk-integration) · [API](#api-overview) · [中文文档](#中文文档)

</div>

---

## Features

| | Feature | Description |
|---|---------|-------------|
| :zap: | **Automatic Instrumentation** | HTTP server/client, gRPC, database/sql middleware out of the box |
| :link: | **Context Propagation** | Seamless trace context across HTTP, gRPC, and Kafka boundaries |
| :dart: | **Adaptive Sampling** | Errors and slow requests always captured, hash-based consistent sampling per trace |
| :rocket: | **Batch Async Export** | SDK buffers spans and sends via gRPC in configurable batches |
| :floppy_disk: | **ClickHouse Storage** | Column-oriented storage with automatic TTL, materialized views for aggregation |
| :mag: | **Query API** | Search traces, service topology, latency/throughput statistics via REST |
| :package: | **Lightweight SDK** | Based on `context.Context`, zero-config defaults, `defer` pattern for span lifecycle |
| :bar_chart: | **Web UI** | Built-in dark-themed dashboard with trace search, waterfall timeline, service topology, and statistics |

## Quick Start

### Docker Compose (recommended)

```bash
# Start ClickHouse + Redis + Collector + Query
cd deploy && docker compose up -d

# Verify services are running
curl http://localhost:28080/health

# Open Web UI
open http://localhost:28080
```

### From Source

```bash
# Build (frontend + Go binaries)
make all

# Or build separately
make web          # Build frontend
make build        # Build Go binaries (embeds frontend via go:embed)

# Run Collector (requires ClickHouse + Redis)
./bin/prism-collector -listen=:24317 -clickhouse=localhost:29000 -redis=localhost:26379

# Run Query Server (serves API + Web UI)
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
| `-metrics` | `:24318` | Prometheus metrics HTTP address |
| `-ingest-token` | (empty) | Bearer token for HTTP ingest auth (empty = no auth) |
| `-cors-origins` | `*` | `Access-Control-Allow-Origin` value for HTTP ingest |
| `-max-buffer` | `100000` | Max spans to buffer in memory before applying backpressure (gRPC → ResourceExhausted, HTTP → 429) |

### Query Server Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-listen` | `:28080` | HTTP listen address |
| `-clickhouse` | `localhost:29000` | ClickHouse native address |
| `-ch-db` | `prism` | ClickHouse database name |
| `-query-token` | (empty) | Bearer token for `/api/v1/*` auth (empty = no auth) |
| `-cors-origins` | `*` | `Access-Control-Allow-Origin` value for query API |

### SDK Options

| Option | Default | Description |
|--------|---------|-------------|
| `WithCollector(addr)` | `localhost:24317` | Collector gRPC address |
| `WithBatchSize(n)` | `1024` | Spans per batch |
| `WithFlushInterval(d)` | `5s` | Batch flush interval |
| `WithSampler(s)` | `AlwaysSampler` | Sampling strategy |
| `WithIngestToken(token)` | (empty) | Bearer token sent in gRPC `authorization` metadata |

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
| `GET` | `/metrics` | Prometheus metrics |

### Observability

```bash
# Collector metrics
curl http://localhost:24318/metrics

# Query metrics
curl http://localhost:28080/metrics

# Grafana dashboard (pre-configured)
open http://localhost:23000   # admin / prism

# Prometheus
open http://localhost:29090
```

## Development

```bash
make all          # Build everything (proto + frontend + binaries)
make web          # Build frontend only
make build        # Build collector + query binaries
make test         # Run tests
make lint         # Run linter
make proto        # Regenerate protobuf code
make docker-up    # Start docker-compose stack
make docker-down  # Stop docker-compose stack
make deps         # Run go mod tidy + npm install
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
├── internal/query/       # Query API handlers + embedded SPA serving
├── internal/storage/     # Storage interface + ClickHouse impl
├── proto/                # Protobuf definitions
├── web/                  # React frontend (Vite + TypeScript + Tailwind)
├── deploy/               # Docker Compose, Dockerfiles, init SQL
└── examples/             # Multi-service demo
```

## Architecture

```
                          ┌─────────────────────────────────────────┐
  ┌──────────┐   gRPC     │            Collector                    │
  │  Go SDK  │──────────▶ │  ┌────────┐  ┌───────┐  ┌───────────┐ │
  └──────────┘            │  │ Receive │─▶│ Batch │─▶│ BatchWrite│ │
  ┌──────────┐   HTTP     │  └────────┘  └───────┘  └─────┬─────┘ │
  │ Any Lang │──────────▶ │                                │       │
  └──────────┘  JSON POST │                                │       │
                          └────────────────────────────────┼───────┘
                                                           │
                          ┌──────────┐              ┌──────▼──────┐
                          │  Redis   │◀─── deps ───│  ClickHouse  │
                          └──────────┘              └──────┬──────┘
                                                           │
                          ┌────────────────────────────────┼───────┐
                          │            Query Server         │       │
                          │  ┌─────────┐  ┌──────────────┐ │       │
                          │  │ REST API│  │ Embedded SPA │ │◀──────┘
                          │  └─────────┘  └──────────────┘ │
                          └────────────────────────────────────────┘
                                          │
                                    ┌─────▼─────┐
                                    │  Browser   │
                                    │  Web UI    │
                                    └───────────┘
```

See [docs/design.md](docs/design.md) for the full technical design document.

## License

MIT

---

<div align="center">

# 中文文档

</div>

## 功能特性

| | 特性 | 说明 |
|---|------|------|
| :zap: | **自动埋点** | HTTP Server/Client、gRPC、database/sql 中间件开箱即用 |
| :link: | **上下文传播** | HTTP Header、gRPC Metadata、Kafka Header 无缝传递 trace 上下文 |
| :dart: | **自适应采样** | 错误和慢请求必采，基于 trace_id hash 的一致性概率采样 |
| :rocket: | **批量异步上报** | SDK 内存缓冲，gRPC 批量发送，可配置 batch 大小和刷新间隔 |
| :floppy_disk: | **ClickHouse 存储** | 列式存储，自带 TTL 过期，物化视图预聚合 |
| :mag: | **查询 API** | 链路搜索、服务拓扑、延迟/吞吐统计，RESTful 接口 |
| :package: | **轻量 SDK** | 基于 `context.Context`，零配置默认值，`defer` 模式管理 Span 生命周期 |
| :bar_chart: | **内置 Web UI** | 暗色主题仪表盘，链路搜索、瀑布流时间线、服务拓扑图、统计图表 |

## 快速开始

### Docker Compose（推荐）

```bash
# 启动 ClickHouse + Redis + Collector + Query
cd deploy && docker compose up -d

# 验证服务运行
curl http://localhost:28080/health

# 打开 Web UI
open http://localhost:28080
```

### 从源码构建

```bash
# 编译（前端 + Go 二进制）
make all

# 或分步编译
make web          # 编译前端
make build        # 编译 Go 二进制（通过 go:embed 内嵌前端）

# 运行 Collector（需要 ClickHouse + Redis）
./bin/prism-collector -listen=:24317 -clickhouse=localhost:29000 -redis=localhost:26379

# 运行 Query 服务（同时提供 API + Web UI）
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
| `-metrics` | `:24318` | Prometheus 指标 HTTP 地址 |
| `-ingest-token` | (空) | HTTP ingest 鉴权 bearer token（空 = 不校验） |
| `-cors-origins` | `*` | HTTP ingest `Access-Control-Allow-Origin` 值 |
| `-max-buffer` | `100000` | 内存 span 缓冲上限，超限对 gRPC 返回 ResourceExhausted，对 HTTP 返回 429 |

### Query 服务参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-listen` | `:28080` | HTTP 监听地址 |
| `-clickhouse` | `localhost:29000` | ClickHouse 原生协议地址 |
| `-ch-db` | `prism` | ClickHouse 数据库名 |
| `-query-token` | (空) | `/api/v1/*` 鉴权 bearer token（空 = 不校验） |
| `-cors-origins` | `*` | Query API `Access-Control-Allow-Origin` 值 |

### SDK 配置项

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `WithCollector(addr)` | `localhost:24317` | Collector gRPC 地址 |
| `WithBatchSize(n)` | `1024` | 每批 Span 数量 |
| `WithFlushInterval(d)` | `5s` | 批量刷新间隔 |
| `WithSampler(s)` | `AlwaysSampler` | 采样策略 |
| `WithIngestToken(token)` | (空) | gRPC `authorization` metadata 中携带的 bearer token |

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
| `GET` | `/metrics` | Prometheus 指标 |

### 可观测性

```bash
# Collector 指标
curl http://localhost:24318/metrics

# Query 指标
curl http://localhost:28080/metrics

# Grafana 仪表盘（预配置，开箱即用）
open http://localhost:23000   # admin / prism

# Prometheus
open http://localhost:29090
```

## 开发命令

```bash
make all          # 编译所有（proto + 前端 + 二进制）
make web          # 仅编译前端
make build        # 编译 collector + query 二进制
make test         # 运行测试
make lint         # 代码检查
make proto        # 重新生成 protobuf 代码
make docker-up    # 启动 docker-compose
make docker-down  # 停止 docker-compose
make deps         # 整理依赖 (go mod tidy + npm install)
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
├── internal/query/       # Query API 处理器 + 内嵌 SPA 服务
├── internal/storage/     # 存储接口 + ClickHouse 实现
├── proto/                # Protobuf 定义
├── web/                  # React 前端 (Vite + TypeScript + Tailwind)
├── deploy/               # Docker Compose、Dockerfile、建表 SQL
└── examples/             # 多服务示例
```

## 详细文档

- [技术设计文档](docs/design.md) — 架构设计、数据模型、模块详解、采样策略、容量估算
- [使用指南](docs/guide.md) — 落地场景、SDK 接入速查、采样配置、部署建议
