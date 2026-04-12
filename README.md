# Bifrost AI Gateway

[![Go Report Card](https://goreportcard.com/badge/github.com/maximhq/bifrost/core)](https://goreportcard.com/report/github.com/maximhq/bifrost/core)
[![Discord badge](https://dcbadge.limes.pink/api/server/https://discord.gg/exN5KAydbU?style=flat)](https://discord.gg/exN5KAydbU)
[![Known Vulnerabilities](https://snyk.io/test/github/maximhq/bifrost/badge.svg)](https://snyk.io/test/github/maximhq/bifrost)
[![codecov](https://codecov.io/gh/maximhq/bifrost/branch/main/graph/badge.svg)](https://codecov.io/gh/maximhq/bifrost)
![Docker Pulls](https://img.shields.io/docker/pulls/maximhq/bifrost)
[<img src="https://run.pstmn.io/button.svg" alt="Run In Postman" style="width: 95px; height: 21px;">](https://app.getpostman.com/run-collection/31642484-2ba0e658-4dcd-49f4-845a-0c7ed745b916?action=collection%2Ffork&source=rip_markdown&collection-url=entityId%3D31642484-2ba0e658-4dcd-49f4-845a-0c7ed745b916%26entityType%3Dcollection%26workspaceId%3D63e853c8-9aec-477f-909c-7f02f543150e)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/bifrost)](https://artifacthub.io/packages/search?repo=bifrost)
[![License](https://img.shields.io/github/license/maximhq/bifrost)](LICENSE)

## The fastest way to build AI applications that never go down

Bifrost is a high-performance AI gateway that unifies access to 15+ providers (OpenAI, Anthropic, AWS Bedrock, Google Vertex, and more) through a single OpenAI-compatible API. Deploy in seconds with zero configuration and get automatic failover, load balancing, semantic caching, and enterprise-grade features.

## Quick Start

![Get started](./docs/media/getting-started.png)

**Go from zero to production-ready AI gateway in under a minute.**

**Step 1:** Start Bifrost Gateway

```bash
# Install and run locally
npx -y @maximhq/bifrost

# Or use Docker
docker run -p 8080:8080 maximhq/bifrost
```

**Step 2:** Configure via Web UI

```bash
# Open the built-in web interface
open http://localhost:8080
```

**Step 3:** Make your first API call

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello, Bifrost!"}]
  }'
```

**That's it!** Your AI gateway is running with a web interface for visual configuration, real-time monitoring, and analytics.

**Complete Setup Guides:**

- [Gateway Setup](https://docs.getbifrost.ai/quickstart/gateway/setting-up) - HTTP API deployment
- [Go SDK Setup](https://docs.getbifrost.ai/quickstart/go-sdk/setting-up) - Direct integration

---

## Enterprise Deployments

Bifrost supports enterprise-grade, private deployments for teams running production AI systems at scale.
In addition to private networking, custom security controls, and governance, enterprise deployments unlock advanced capabilities including adaptive load balancing, clustering, guardrails, MCP gateway and and other features designed for enterprise-grade scale and reliability.

<img src=".github/assets/features.png" alt="Book a Demo" width="100%" style="margin-top:5px;"/>


<div align="center" style="display: flex; flex-direction: column;">
  <a href="https://calendly.com/maximai/bifrost-demo">
    <img src=".github/assets/book-demo-button.png" alt="Book a Demo" width="170" style="margin-top:5px;"/>
  </a>
  <div>
  <a href="https://www.getmaxim.ai/bifrost/enterprise" target="_blank" rel="noopener noreferrer">Explore enterprise capabilities</a>
  </div>
</div>

---

## Key Features

### Core Infrastructure

- **[Unified Interface](https://docs.getbifrost.ai/features/unified-interface)** - Single OpenAI-compatible API for all providers
- **[Multi-Provider Support](https://docs.getbifrost.ai/quickstart/gateway/provider-configuration)** - OpenAI, Anthropic, AWS Bedrock, Google Vertex, Azure, Cerebras, Cohere, Mistral, Ollama, Groq, and more
- **[Automatic Fallbacks](https://docs.getbifrost.ai/features/fallbacks)** - Seamless failover between providers and models with zero downtime
- **[Load Balancing](https://docs.getbifrost.ai/features/fallbacks)** - Intelligent request distribution across multiple API keys and providers

### Advanced Features

- **[Model Context Protocol (MCP)](https://docs.getbifrost.ai/features/mcp)** - Enable AI models to use external tools (filesystem, web search, databases)
- **[Semantic Caching](https://docs.getbifrost.ai/features/semantic-caching)** - Intelligent response caching based on semantic similarity to reduce costs and latency
- **[Multimodal Support](https://docs.getbifrost.ai/quickstart/gateway/streaming)** - Support for text,images, audio, and streaming, all behind a common interface.
- **[Custom Plugins](https://docs.getbifrost.ai/enterprise/custom-plugins)** - Extensible middleware architecture for analytics, monitoring, and custom logic
- **[Governance](https://docs.getbifrost.ai/features/governance)** - Usage tracking, rate limiting, and fine-grained access control

### Enterprise & Security

- **[Budget Management](https://docs.getbifrost.ai/features/governance)** - Hierarchical cost control with virtual keys, teams, and customer budgets
- **[SSO Integration](https://docs.getbifrost.ai/features/sso-with-google-github)** - Google and GitHub authentication support
- **[Observability](https://docs.getbifrost.ai/features/observability)** - Native Prometheus metrics, distributed tracing, and comprehensive logging
- **[Vault Support](https://docs.getbifrost.ai/enterprise/vault-support)** - Secure API key management with HashiCorp Vault integration

### Developer Experience

- **[Zero-Config Startup](https://docs.getbifrost.ai/quickstart/gateway/setting-up)** - Start immediately with dynamic provider configuration
- **[Drop-in Replacement](https://docs.getbifrost.ai/features/drop-in-replacement)** - Replace OpenAI/Anthropic/GenAI APIs with one line of code
- **[SDK Integrations](https://docs.getbifrost.ai/integrations/what-is-an-integration)** - Native support for popular AI SDKs with zero code changes
- **[Configuration Flexibility](https://docs.getbifrost.ai/quickstart/gateway/provider-configuration)** - Web UI, API-driven, or file-based configuration options

---

## Repository Structure

Bifrost uses a modular architecture for maximum flexibility:

```text
bifrost/
├── npx/                 # NPX script for easy installation
├── core/                # Core functionality and shared components
│   ├── providers/       # Provider-specific implementations (OpenAI, Anthropic, etc.)
│   ├── schemas/         # Interfaces and structs used throughout Bifrost
│   └── bifrost.go       # Main Bifrost implementation
├── framework/           # Framework components for data persistence
│   ├── configstore/     # Configuration storages
│   ├── logstore/        # Request logging storages
│   └── vectorstore/     # Vector storages
├── transports/          # HTTP gateway and other interface layers
│   └── bifrost-http/    # HTTP transport implementation
├── ui/                  # Web interface for HTTP gateway
├── plugins/             # Extensible plugin system
│   ├── governance/      # Budget management and access control
│   ├── jsonparser/      # JSON parsing and manipulation utilities
│   ├── logging/         # Request logging and analytics
│   ├── maxim/           # Maxim's observability integration
│   ├── mocker/          # Mock responses for testing and development
│   ├── semanticcache/   # Intelligent response caching
│   └── telemetry/       # Monitoring and observability
├── docs/                # Documentation and guides
└── tests/               # Comprehensive test suites
```

---

## Getting Started Options

Choose the deployment method that fits your needs:

### 1. Gateway (HTTP API)

**Best for:** Language-agnostic integration, microservices, and production deployments

```bash
# NPX - Get started in 30 seconds
npx -y @maximhq/bifrost

# Docker - Production ready
docker run -p 8080:8080 -v $(pwd)/data:/app/data maximhq/bifrost
```

**Features:** Web UI, real-time monitoring, multi-provider management, zero-config startup

**Learn More:** [Gateway Setup Guide](https://docs.getbifrost.ai/quickstart/gateway/setting-up)

### 2. Go SDK

**Best for:** Direct Go integration with maximum performance and control

```bash
go get github.com/maximhq/bifrost/core
```

**Features:** Native Go APIs, embedded deployment, custom middleware integration

**Learn More:** [Go SDK Guide](https://docs.getbifrost.ai/quickstart/go-sdk/setting-up)

### 3. Drop-in Replacement

**Best for:** Migrating existing applications with zero code changes

```diff
# OpenAI SDK
- base_url = "https://api.openai.com"
+ base_url = "http://localhost:8080/openai"

# Anthropic SDK  
- base_url = "https://api.anthropic.com"
+ base_url = "http://localhost:8080/anthropic"

# Google GenAI SDK
- api_endpoint = "https://generativelanguage.googleapis.com"
+ api_endpoint = "http://localhost:8080/genai"
```

**Learn More:** [Integration Guides](https://docs.getbifrost.ai/integrations/what-is-an-integration)

---

## Performance

Bifrost adds virtually zero overhead to your AI requests. In sustained 5,000 RPS benchmarks, the gateway added only **11 µs** of overhead per request.

| Metric | t3.medium | t3.xlarge | Improvement |
|--------|-----------|-----------|-------------|
| Added latency (Bifrost overhead) | 59 µs | **11 µs** | **-81%** |
| Success rate @ 5k RPS | 100% | 100% | No failed requests |
| Avg. queue wait time | 47 µs | **1.67 µs** | **-96%** |
| Avg. request latency (incl. provider) | 2.12 s | **1.61 s** | **-24%** |

**Key Performance Highlights:**

- **Perfect Success Rate** - 100% request success rate even at 5k RPS
- **Minimal Overhead** - Less than 15 µs additional latency per request
- **Efficient Queuing** - Sub-microsecond average wait times
- **Fast Key Selection** - ~10 ns to pick weighted API keys

**Complete Benchmarks:** [Performance Analysis](https://docs.getbifrost.ai/benchmarking/getting-started)

---

## Documentation

**Complete Documentation:** [https://docs.getbifrost.ai](https://docs.getbifrost.ai)

### Quick Start

- [Gateway Setup](https://docs.getbifrost.ai/quickstart/gateway/setting-up) - HTTP API deployment in 30 seconds
- [Go SDK Setup](https://docs.getbifrost.ai/quickstart/go-sdk/setting-up) - Direct Go integration
- [Provider Configuration](https://docs.getbifrost.ai/quickstart/gateway/provider-configuration) - Multi-provider setup

### Features

- [Multi-Provider Support](https://docs.getbifrost.ai/features/unified-interface) - Single API for all providers
- [MCP Integration](https://docs.getbifrost.ai/features/mcp) - External tool calling
- [Semantic Caching](https://docs.getbifrost.ai/features/semantic-caching) - Intelligent response caching
- [Fallbacks & Load Balancing](https://docs.getbifrost.ai/features/fallbacks) - Reliability features
- [Budget Management](https://docs.getbifrost.ai/features/governance) - Cost control and governance

### Integrations

- [OpenAI SDK](https://docs.getbifrost.ai/integrations/openai-sdk) - Drop-in OpenAI replacement
- [Anthropic SDK](https://docs.getbifrost.ai/integrations/anthropic-sdk) - Drop-in Anthropic replacement
- [AWS Bedrock SDK](https://docs.getbifrost.ai/integrations/bedrock-sdk) - AWS Bedrock integration
- [Google GenAI SDK](https://docs.getbifrost.ai/integrations/genai-sdk) - Drop-in GenAI replacement
- [LiteLLM SDK](https://docs.getbifrost.ai/integrations/litellm-sdk) - LiteLLM integration
- [Langchain SDK](https://docs.getbifrost.ai/integrations/langchain-sdk) - Langchain integration

### Enterprise

- [Custom Plugins](https://docs.getbifrost.ai/enterprise/custom-plugins) - Extend functionality
- [Clustering](https://docs.getbifrost.ai/enterprise/clustering) - Multi-node deployment
- [Vault Support](https://docs.getbifrost.ai/enterprise/vault-support) - Secure key management
- [Production Deployment](https://docs.getbifrost.ai/deployment/docker-setup) - Scaling and monitoring

---

## Need Help?

**[Join our Discord](https://discord.gg/exN5KAydbU)** for community support and discussions.

Get help with:

- Quick setup assistance and troubleshooting
- Best practices and configuration tips  
- Community discussions and support
- Real-time help with integrations

---

## Contributing

We welcome contributions of all kinds! See our [Contributing Guide](https://docs.getbifrost.ai/contributing/setting-up-repo) for:

- Setting up the development environment
- Code conventions and best practices
- How to submit pull requests
- Building and testing locally

For development requirements and build instructions, see our [Development Setup Guide](https://docs.getbifrost.ai/contributing/setting-up-repo#development-environment-setup).

---

## License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.

Built with ❤️ by [Maxim](https://github.com/maximhq)

---

## Fork Enterprise Customization Notes

### 2026-04-05 07:43:04 CST | Base Commit 44afab82 | Round 1

- Added enterprise transport config models and schema wiring for `cluster_config`, `load_balancer_config`, `audit_logs`, `alerts`, `log_exports`, and `vault`.
- Implemented adaptive key-level load balancing using real-time route metrics with EWMA-based latency/error scoring and dynamic key weight adjustment.
- Registered the adaptive load balancer as a built-in runtime component and wired its `KeySelector` into Bifrost initialization.
- Added validation coverage for enterprise config parsing and adaptive load-balancer behavior.
- Added Vault-related configuration scaffolding for future secret synchronization work. Runtime vault sync was not implemented in this round.

### 2026-04-05 07:43:04 CST | Base Commit 44afab82 | Round 2

- Implemented a static-peer cluster synchronization service on top of `KVStore.SyncDelegate`, including internal KV mutation replication and peer health checks.
- Added enterprise APIs for cluster state and internal replication endpoints:
  - `GET /api/cluster/status`
  - `GET /_cluster/status`
  - `POST /_cluster/kv/set`
  - `POST /_cluster/kv/delete`
- Added append-only audit logging with chained integrity hashes and optional HMAC signing, exposed through `GET /api/audit-logs`.
- Added enterprise log export jobs for both LLM logs and MCP logs with `jsonl`/`csv` output and optional `gzip` compression:
  - `POST /api/logs/exports`
  - `POST /api/mcp-logs/exports`
  - `GET /api/log-exports`
  - `GET /api/log-exports/{id}`
- Added an alert manager for budget threshold, error-rate, and average-latency alerts with Email, Feishu, and generic Webhook delivery, exposed through `GET /api/alerts`.
- Added enterprise unit tests covering audit logging, export job generation, and cluster mutation application.

Current implementation status:

- Adaptive load balancing is functional.
- Cluster mode currently supports static peers plus health polling and KV replication.
- Audit logs, log exports, and alerts are functional.
- Vault runtime synchronization and automatic discovery backends for cluster mode are still pending.

### 2026-04-05 15:37:40 CST | Base Commit d4bcbed8 | Consolidated Follow-up Work

- Hardened cluster runtime behavior with cluster token authentication, peer health thresholds, richer peer metadata, and runtime-versus-ConfigStore fingerprinting for drift detection.
- Added dynamic cluster discovery for `dns` and `kubernetes`, including periodic peer refresh, stale discovered-peer cleanup, and discovery status reporting in cluster health views.
- Extended adaptive routing from key-level metrics to route-plus-direction metrics, with provider/model direction health snapshots, fallback reordering based on direction health, and a new `/api/adaptive-routing/status` surface.
- Added cluster-aware read aggregation for adaptive routing status, alerts, audit logs, and log export jobs so operators can inspect multiple nodes from a single dashboard view.
- Implemented Vault runtime synchronization for HashiCorp Vault KV and in-cluster Kubernetes Secrets, including env-backed provider hot refresh, optional auto-deprecate behavior for removed secrets, and `/api/vault/status`.
- Improved audit and export safety by preferring real user identity in audit records, hashing session fallback identifiers, persisting export job metadata across restarts, and adding export file download support.
- Added controlled cluster config propagation and hot reload for these scopes: `client`, `auth`, `framework`, `proxy`, `provider`, `mcp_client`, `customer`, `team`, and `virtual_key`.
- Added peer-side governance apply logic for `customers`, `teams`, and `virtual keys`, including dependency waiting and stable MCP client resolution for virtual-key MCP bindings.
- Replaced the enterprise placeholder pages for Cluster Mode, Adaptive Routing, and Audit Logs with working UI views backed by real APIs, cluster-aware summaries, drift indicators, alert visibility, and export management.
- Added UI/runtime support so enterprise status endpoints remain registered even when the corresponding service is disabled, allowing the UI to show explicit “not enabled” states instead of generic 404-based errors.
- Added a UI embed fallback for default test and dev binaries, while preserving production UI embedding through the `embedui` build path in the main binary, Docker images, and Nix packaging.
- Added regression coverage for cluster auth, discovery refresh, config propagation, governance replication, disabled enterprise status routes, export persistence, audit behavior, Vault sync, and cluster-aware aggregation behavior.

Current supported enterprise scope after today’s work:

- Adaptive load balancing is functional at key, route, and direction visibility layers.
- Cluster mode supports static peers, DNS/Kubernetes discovery, health polling, KV replication, config drift inspection, and controlled hot propagation for the core runtime/governance scopes listed above.
- Audit logs, alerting, log exports, Vault runtime sync, and the corresponding enterprise UI pages are functional.

Still intentionally not completed:

- Consul/etcd discovery backends and a fuller gossip-based shared state plane.
- Frontend still carries a few historical placeholder hooks for standalone `budget` / `rate-limit` mutations, but effective budget and rate-limit writes already flow through the synchronized governance resources (`virtual key / team / customer / model config / provider governance`) instead of a separate backend mutation surface.
- AWS Secrets Manager, GCP Secret Manager, and Azure Key Vault backends.
- RBAC backend APIs, MCP with federated auth, and other enterprise-only governance surfaces that are still placeholder/fallback oriented.

### 2026-04-05 21:32:51 CST | Base Commit d8b5f6fa2 | Cluster Consistency Follow-up

- Added cluster propagation and peer-side apply logic for OAuth state so MCP OAuth flows remain consistent across nodes:
  - OAuth config changes now sync after initiation, pending MCP client persistence, callback success/failure, refresh, and revoke.
  - OAuth token create/update/delete now propagates to peers through the existing authenticated `/_cluster/config/reload` path.
- Added prompt-repository cluster synchronization for `folders`, `prompts`, `prompt versions`, and `prompt sessions`.
  - Prompt repository writes now fan out from the originating node.
  - Peer nodes apply prompt repo state with preserved identifiers, timestamps, model params, and message ordering so dashboards stay consistent across nodes.
- Added dashboard session replication for cluster deployments.
  - Successful `/api/session/login` now propagates created sessions to peers.
  - `/api/session/logout` now propagates session invalidation to peers.
  - This removes the previous requirement that dashboard traffic remain sticky to the node that created the session when nodes use separate local config stores.
- Hardened WebSocket dashboard auth behavior for clustered deployments.
  - If a short-lived `ws-ticket` is issued on one node but the WebSocket upgrade lands on another node, auth now falls back to the synchronized session cookie instead of failing immediately.
  - This keeps dashboard log-stream and live-update flows working correctly behind load balancers without requiring ticket-store replication.
- Extended Cluster Mode observability to include prompt repository resource counts in the cluster status UI so the visible “Tracked Resources” summary matches the expanded sync scope.

Validation completed for this follow-up:

- `go test ./framework/oauth2 ./transports/bifrost-http/lib -run TestDoesNotExist`
- `go test ./transports/bifrost-http/handlers ./transports/bifrost-http/server ./transports/bifrost-http/enterprise ./transports/bifrost-http/integrations ./transports/bifrost-http/loadbalancer ./transports/bifrost-http/websocket`
- `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
- `npm exec next build -- --no-lint`
- `npm exec tsc -- --noEmit`

Current cluster auto-sync scope now includes:

- `client`, `auth`, `framework`, `proxy`, `provider`, and `provider governance`
- `mcp client`, OAuth state, dashboard sessions, and built-in plugins
- `customer`, `team`, `virtual key`, `model config`, and `routing rule`
- prompt repository `folder`, `prompt`, `prompt version`, and `prompt session`

### 2026-04-05 21:41:06 CST | Base Commit 28a3f8dce | 集群前后端联动与高可用补强

- 补齐了集群配置变更后的前端自动刷新链路。
  - 现在节点本地配置变更成功后，会通过现有 WebSocket `store_update` 消息主动广播对应的 RTK Query tags。
  - peer 节点在应用 `/_cluster/config/reload` 变更成功后，也会向本节点已连接的前端页面广播同样的 tags。
  - 这样浏览器无论连在哪个节点，只要任意节点完成配置更新，相关页面都会自动触发失效与重新拉取，而不需要手动刷新。

- 为不同配置域补充了精确的 UI 失效标签映射，覆盖：
  - `provider / db keys / models / base models`
  - `customer / team / virtual key / budget / rate limit / model config / provider governance / routing rule`
  - `mcp client / OAuth2Config`
  - `plugin`
  - `prompt repository` 的 `folder / prompt / version / session`
  - `cluster status`

- 新增 `SessionState` 前端缓存标签，用于更稳定地处理登录态变化。
  - `GET /api/session/is-auth-enabled` 现在会提供 `SessionState` tag。
  - 登录、登出以及集群侧 session 同步后，相关页面可以更可靠地自动刷新登录态。

- 进一步增强了集群高可用下的 Dashboard/WebSocket 体验。
  - 当 `ws-ticket` 在一个节点签发、WebSocket 升级请求被负载均衡打到另一个节点时，认证现在会继续回退到已同步的 session cookie。
  - 结合本轮前面已经完成的 session 集群同步，可以避免因节点切换导致的 Dashboard 实时日志/状态流中断。

本轮验证通过：

- `go test ./framework/oauth2 ./transports/bifrost-http/lib -run TestDoesNotExist`
- `go test ./transports/bifrost-http/handlers ./transports/bifrost-http/server ./transports/bifrost-http/enterprise ./transports/bifrost-http/integrations ./transports/bifrost-http/loadbalancer ./transports/bifrost-http/websocket`
- `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
- `npm exec next build -- --no-lint`
- `npm exec tsc -- --noEmit`

### 2026-04-05 21:59:03 CST | Base Commit 6a015f91d | 剩余治理同步核查与 WebSocket 回归补强

- 重新核查了治理面剩余的写入口，确认当前后端并不存在独立的 `budget` / `rate-limit` 写接口。
  - 这两类能力的实际写路径都挂在已经完成集群同步的对象之下：`virtual key / team / customer / model config / provider governance`。
  - 这也意味着多节点环境下，预算和限流配置的生效链路已经跟随这些治理对象自动同步，而不是额外依赖另一套未实现的后台接口。

- 补强了配置变更后的 WebSocket `store_update` 回归测试，锁住两类高可用场景：
  - 源节点在向 peer fanout 失败时，仍然会先向本节点前端页面广播缓存失效 tags，避免运维页面因为某个 peer 故障而卡在旧数据。
  - peer 节点在成功应用 `/_cluster/config/reload` 之后，会向本节点已连接页面广播对应 tags，保证跨节点生效后 UI 立即刷新。

- 新增了更完整的 cluster scope -> RTK tag 覆盖性测试。
  - 现在所有已支持的集群同步 scope 都会被校验必须产生非空的 UI 失效标签，并统一包含 `ClusterNodes`，防止后续新增 scope 时漏掉前端刷新联动。

- 文档层同步纠正了较早一轮里已经过时的“未完成项”描述，避免把已经落地的 `routing rule / model config / provider governance` 同步能力继续误记为未完成。

本轮验证通过：

- `go test ./transports/bifrost-http/server`
- `go test ./transports/bifrost-http/handlers ./transports/bifrost-http/enterprise ./transports/bifrost-http/integrations ./transports/bifrost-http/loadbalancer ./transports/bifrost-http/websocket`
- `go test ./framework/oauth2 ./transports/bifrost-http/lib -run TestDoesNotExist`
- `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
- `npm exec next build -- --no-lint`
- `npm exec tsc -- --noEmit`

### 2026-04-06 15:11:18 CST | Base Commit 512c6c6c0 | 集群同步稳定性、Cluster 视图语义与 Virtual Key 交互修复

- 修复了集群 `provider` 热同步在 peer 节点上的关键稳定性问题。
  - 之前 peer 节点在应用 `provider` 配置时，如果本地 `ModelCatalog / PricingManager` 尚未初始化，会直接报 `pricing manager not found`，导致共享 `ConfigStore` 已更新但运行时未热生效，进而在 Cluster 页里持续表现为 `Runtime Drift`。
  - 现在 `ReloadProvider` 会先尝试按当前 `framework.pricing` 配置懒初始化 `ModelCatalog`；如果部署本身没有启用 pricing/model catalog，也不会因为缺少 pricing manager 而让 provider 热加载失败。
  - `RemoveProvider` 也不再因为没有 `ModelCatalog` 而中断删除链路，避免 peer 已经删掉 provider 但清理 pricing 缓存时反向报错。

- 修复了共享 `ConfigStore` 场景下 provider 删除的幂等性问题。
  - 之前一个节点先删掉 provider 后，其他节点收到 cluster reload 再去删同一条数据库记录时，会因为 `DeleteProvider -> not found` 被当成错误，出现“Client and config may be out of sync”的误导日志。
  - 现在 provider 删除对 `configstore.ErrNotFound` 做了幂等处理，peer 节点会把“数据库里已经不存在”视为合法状态，只继续清理本地运行时缓存。

- 补强了 cluster reload 的来源可观测性。
  - `ClusterConfigChange` 现在会携带 `source_node_id`。
  - peer 节点在接收并应用 `/_cluster/config/reload` 时，日志会带上 `source_node=<pod-or-node-id>`，不再只能看到对端连接的临时 `ip:port`。
  - 这能更快定位到底是哪一个节点发出的 provider / virtual key / governance 变更。

- 调整了 Cluster 页的统计语义和展示结构，避免误判。
  - 原来的 `Peers` 实际只统计“远端 peer”，不包含当前访问节点，所以 3 个 Pod 的集群会显示成 2；现在汇总卡片新增并优先展示 `Cluster Nodes` 总数，同时保留 `Remote Peers`。
  - `Discovered Peers` 改成更明确的 `Dynamic Peers` 语义，避免静态 `peers + headless service` 场景下看到 0 产生误解。
  - 新增按 Pod 维度的 `Node Cards` 视图，直接展示本地节点和每个远端节点的 `node id / started time / config sync / runtime match / resolved from / last error`，不再只能依赖下方表格判断状态。
  - `Remote Peer Status` 表格保留，用于继续查看成功次数、失败次数、最近错误等细节。

- 优化了 Cluster 页里 `Runtime Drift` 的解释文案。
  - 现在页面会明确说明：`Runtime Drift` 不是单纯的页面刷新延迟，而是“某个节点当前正在服务的内存运行时配置，与最新 ConfigStore 快照或其他节点的运行时指纹不一致”。
  - 这类提示通常意味着热加载失败、未完成，或者某个节点运行时没有正确刷新，而不只是同步稍慢。

- 修复了 `Virtual Key` 列表页“点击复制 key 报 failed copy”的问题。
  - 之前复制逻辑只依赖 `navigator.clipboard.writeText`，在某些浏览器权限、非安全上下文或特定嵌入环境下会直接失败。
  - 现在增加了 `textarea + document.execCommand("copy")` 的兼容性兜底，并确保点击复制/显示按钮时不会误触发行点击。

本轮验证通过：

- `go test ./transports/bifrost-http/server ./transports/bifrost-http/handlers ./transports/bifrost-http/enterprise`
- `go test ./transports/bifrost-http/lib -run 'TestRemoveProvider_(Success|NotFound|DBError_DoesNotRemoveFromMemory|DBNotFound_IsIdempotent|NilConfigStore_RemovesFromMemoryOnly|SkipDBUpdate)$'`
- `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
- `npm exec next build -- --no-lint`
- `npm exec tsc -- --noEmit`

### 2026-04-06 15:37:28 CST | Base Commit 512c6c6c0 | 受控低频 ConfigStore 自愈 reload

- 补上了一个“只在检测到 drift 时触发、低频、带冷却去重”的 ConfigStore 自愈机制。
  - 不引入高频数据库 watcher。
  - 仅在本节点 `runtime hash != store hash` 且存在可安全处理的漂移域时，才会启动自愈。
  - 自愈 worker 会对同一份 `store_hash + drift_domains` 做冷却去重，避免在同一批未恢复的漂移上反复刷 reload。

- 当前纳入自愈范围的是最稳妥的核心配置域：
  - `client`
  - `auth`
  - `framework`
  - `proxy`
  - `providers`
  - `governance`
  - 这意味着当某个节点因为 cluster fanout 丢失、临时热加载失败、或运行时缓存未及时刷新而落后于共享 `ConfigStore` 时，会在后续低频检查中自动从 `ConfigStore` 拉取并补齐本地 runtime。

- 自愈 reload 采用“只读 ConfigStore、只改本节点 runtime”的方式，确保幂等且不会反向污染共享存储。
  - `auth` 现在支持直接从 `ConfigStore` 重新加载到 `AuthMiddleware` 和 runtime snapshot，不会再次写回数据库，也不会误触发 session flush。
  - `client` 自愈会同时刷新 `client config / header filter / MCP tool manager runtime`，保持和正常配置更新后的行为一致。
  - `providers` 自愈会对比 store/runtime 的 provider 指纹，仅对真正漂移的 provider 做本地 add/update/remove，并通过 `skip DB update` 上下文保证不会重复写库。
  - `governance` 自愈会按对象粒度对 `customer / team / virtual key / model config / routing rule / provider governance` 做本地 reload 或清理，避免为了恢复一个对象就粗暴重建整套治理插件。

- 自愈完成后会向本节点前端页面广播 `store_update` 标签。
  - Cluster、Providers、Virtual Keys、Governance 等页面在本节点被动恢复 runtime 后，也能及时刷新展示，不需要手动 reload 页面。

- 这轮也把相关幂等与去重测试补上了。
  - 校验同一份 drift signature 在冷却窗口内不会重复触发自愈。
  - 校验 runtime 已经 in-sync 时不会误触发自愈。
  - 校验 `auth` 自愈 reload 只刷新 runtime，不会向 `ConfigStore` 再次写入，并且重复执行仍然保持幂等。

本轮验证通过：

- `go test ./transports/bifrost-http/server`
- `go test ./transports/bifrost-http/handlers ./transports/bifrost-http/enterprise ./transports/bifrost-http/server`
- `go test ./transports/bifrost-http/...`
- `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
- `npm exec next build -- --no-lint`
- `npm exec tsc -- --noEmit`

### 2026-04-06 16:43:14 CST | Base Commit 512c6c6c0 | LLM Logs 输入输出内容回归修复

- 修复了最近 cluster / self-heal 引入的一处 runtime reload 回归。
  - 根因不是日志详情页前端渲染丢字段，而是 `ReloadClientConfigFromConfigStore` 之前直接替换了 `s.Config.ClientConfig` 指针。
  - 但内置 `logging` 和 `governance` 插件在初始化时拿的是 `ClientConfig` 内部字段地址，后续 client reload 如果直接换对象，这两个插件会继续引用旧的 client config。
  - 在这种情况下，只要旧对象里的 `disable_content_logging`、`logging_headers`、`enforce_auth_on_inference`、`required_headers` 与当前 store/runtime 不一致，就会出现“配置看起来已更新，但插件实际还在按旧值工作”的问题。

- 这次把 client runtime reload 改成了“原地更新 + 统一重绑”。
  - `ReloadClientConfigFromConfigStore` 不再直接替换 `ClientConfig` 对象；如果 runtime 已有实例，就执行原地覆盖，尽量保留现有 live 引用。
  - 同时新增了对依赖 client 配置的内置插件的显式重绑逻辑，当前覆盖：
    - `logging`
    - `governance`
  - 这样即使节点此前已经经历过一次“错误的指针替换”，后续只要再触发一次 client config reload，就能把插件重新绑回当前正确的 runtime config。

- 为 `logging` 和 `governance` 插件补了显式的 `BindClientConfig` 能力。
  - `logging` 现在可以在 runtime reload 后重新绑定：
    - `disable_content_logging`
    - `logging_headers`
  - `governance` 现在可以在 runtime reload 后重新绑定：
    - `enforce_auth_on_inference`
    - `required_headers`
  - 这两个重绑方法都是幂等的，重复执行不会引入额外副作用。

- 新增了回归测试，直接覆盖“插件绑在旧 client config 上，再从 ConfigStore reload”的真实场景。
  - 测试会先构造一个 stale client config，让 `logging` / `governance` 故意绑定到旧对象。
  - 再通过 server 触发两次 `ReloadClientConfigFromConfigStore`，验证：
    - 插件最终会绑定到新的 store/runtime 配置
    - 连续 reload 两次结果保持一致
    - 整个 reload 过程保持幂等

- 另外补了一条更贴近实际症状的 logging 插件回归测试。
  - 直接验证 logging 插件在最初绑定到错误的 stale client config 后，只要执行一次显式重绑，就会重新恢复输入/输出内容写入，而不是只验证内部绑定值发生变化。
  - 这样可以更直接地覆盖“LLM Logs 里没有输入和输出内容”的实际故障表现。

本轮验证通过：

- `go test ./transports/bifrost-http/server -run 'TestReloadClientConfigFromConfigStoreRebindsClientConfigDependentPlugins|TestClusterConfigSelfHealerRunOnceDeduplicatesSameSignature|TestReloadAuthConfigFromConfigStoreUpdatesRuntimeOnlyAndIsIdempotent'`
- `go test ./plugins/logging -run 'TestBindClientConfigRebindsContentLoggingBehavior|TestUpdateLogEntrySuppressesChatOutputWhenContentLoggingDisabled|TestUpdateLogEntryUpdatesContentSummaryForChatOutput'`
- `go test ./plugins/logging ./plugins/governance ./transports/bifrost-http/server`
- `go test ./transports/bifrost-http/handlers ./transports/bifrost-http/enterprise -run TestDoesNotExist`
- `go test ./transports/bifrost-http/server ./transports/bifrost-http/handlers ./transports/bifrost-http/enterprise ./transports/bifrost-http/integrations`
- `go test ./transports/bifrost-http/loadbalancer ./transports/bifrost-http/websocket`
- `go test ./transports/bifrost-http/lib -run TestDoesNotExist`
- `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
- `npm exec next build -- --no-lint`

补充说明：

- `go test ./transports/bifrost-http/...` 和 `go test ./transports/bifrost-http/lib ./transports/bifrost-http/loadbalancer ./transports/bifrost-http/websocket` 在当前环境里依旧出现过长时间无输出的现象，因此这轮采用了关键子包拆分验证，不拿挂起中的总命令去冒充完整通过。
- `npm exec tsc -- --noEmit` 在当前仓库环境里仍然会因为 `.next/types` 产物引用不完整而失败，这属于已有的前端类型产物问题；本轮 `next build` 已通过，说明前端构建链路没有被这次修复打坏。

### 2026-04-12 00:48:25 CST | Base Commit 90fbe5f7c | 社区版本对齐与品牌更新

- 本轮重新检索并对比了最近几个社区版本：
  - `transports/v1.4.20`
  - `transports/v1.4.21`
  - `transports/v1.4.22`
  - `transports/v1.5.0-prerelease1`
  - `transports/v1.5.0-prerelease2`

- 对比结果分成三类处理：
  - 已经在当前 fork 中具备或等价覆盖的能力，不重复硬并：
    - Realtime 支持
    - Fireworks provider
    - Prompt Repo / Prompt Session 相关能力
    - Server bootstrap timer
    - 路由白名单 / 安全路径控制
  - 与当前企业定制主链路耦合较深、直接迁入风险较高的能力，暂不贸然合并：
    - Access Profiles
    - Per-user OAuth consent 的整套权限面
    - 依赖更完整治理模型的 business unit 相关扩展
  - 本轮选择性合入、并且已经完成兼容验证的社区更新：
    - 日志治理追踪字段
    - 相关持久化、查询和详情展示能力

- 本轮实际合入的社区更新内容：
  - 将日志链路对齐到社区近期版本里更完整的治理上下文追踪能力。
    - LLM Logs 现在会持久化并返回：
      - `user_id`
      - `team_id`
      - `team_name`
      - `customer_id`
      - `customer_name`
    - 这些字段来自现有 governance context，不改变既有 virtual key / team / customer 解析逻辑。
  - `logstore` 已补充对应数据库列和索引，并通过显式 migration 落库。
    - 新增字段不会破坏旧日志读取。
    - 在 SQLite / Postgres 场景下都保持向下兼容。
  - `logstore` 查询侧已经支持按 `user/team/customer` 三类标识过滤。
    - 同时为了保证统计正确性，materialized view 路径在带这些治理过滤条件时会自动回退到原始 logs 表，不会拿不完整的聚合视图做错误统计。
  - 日志详情页已补充 `Governance Context` 展示块。
    - 可以直接看到当前请求关联的 `User ID / Team / Customer`。

- 本轮同时做了品牌位调整：
  - Sidebar 与登录页原有 logo 区域已改为文本品牌 `TCL华星`。
  - 不再依赖原始 Bifrost logo 图片。

- 本轮社区对齐后的最新参考版本号：
  - `transports/v1.5.0-prerelease2`
  - 说明：这里采用的是“选择性安全合并”策略，不是整 tag 生硬覆盖，以保证此前已经完成的企业级 cluster / governance / vault / audit / export 定制逻辑不被回滚或打乱。

- 本轮验证通过：
  - `go test ./framework/logstore`
  - `go test ./plugins/logging`
  - `go test ./transports/bifrost-http/handlers -run TestDoesNotExist`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
  - `npm exec next build -- --no-lint`
  - `npm exec tsc -- --noEmit`

### 2026-04-12 | Base Commit 0ce12c6 | MCP 联合身份验证 + API 导入 + 日志下载修复

- **MCP 联合身份验证 (Federated Auth) 支持**
  - 在 MCP 工具执行路径 (`core/mcp/toolmanager.go`) 中新增请求级模板变量解析。
  - 支持 `{{req.header.<name>}}`：从调用方的 HTTP 请求头中动态提取认证信息并转发给 MCP 工具服务端。
  - 支持 `{{env.<VAR>}}`：从环境变量中读取值。
  - 零信任架构：认证信息不缓存，每次请求独立认证，直接透传到目标 API。
  - 支持的认证模式：Bearer Token (JWT/OAuth)、API Key (header/query)、Custom Headers (tenant ID, user token)、Basic Auth。

- **MCP API 导入功能**
  - 新增 "Import APIs" 按钮（MCP Registry 页面）打开多标签页导入对话框：
  - **Postman Collection**：粘贴 Postman Collection v2.1 JSON → 解析所有请求 → 预览表格 → 批量导入为 MCP 工具。
  - **OpenAPI Spec**：粘贴 OpenAPI 3.0+ JSON/YAML → 自动解析 paths + security schemes → 转换为 MCP 工具。支持 Bearer Auth 和 API Key 安全方案自动转换为 `{{req.header.*}}` 模板。
  - **cURL Commands**：粘贴一个或多个 cURL 命令 → 解析 `-X`/`-H`/`-d`/URL → 转换为 MCP 工具。
  - **Manual API Builder**：内置 UI 手动配置：HTTP Method + URL + Headers（支持模板变量提示）+ Request Body → 一键添加为 MCP 工具。
  - 所有导入方式均保留 `{{req.header.*}}` 模板变量，确保联合认证生效。
  - 新增文件：`ui/app/workspace/mcp-registry/views/mcpImportDialog.tsx`

- **日志导出下载 "file already closed" 错误修复**
  - 根因：`defer file.Close()` 在 `SetBodyStream` 返回后执行，但 fasthttp 异步读取 stream，导致文件已关闭。
  - 修复：改用 `io.ReadFull` 同步读取文件到内存，然后用 `SetBody` 发送响应。

- **验证通过**
  - `go build ./...` (core) ✓
  - `go test ./mcp/...` (core) ✓
  - `npx tsc --noEmit` ✓

### 2026-04-12 | Base Commit a237336 | 日志导出企业级增强 — 定时导出 + 云存储 + 配置管理

- **日志导出系统全面升级，对齐官网企业版文档设计**
  - 参考文档：https://docs.getbifrost.ai/enterprise/log-exports
  - 从"手动一次性导出"升级为"可配置定时导出 + 多目标存储"的企业级数据导出系统。

- **命名导出配置 (LogExportConfig)**
  - 新增 `log_export_configs` 数据库表，支持创建多个独立的命名导出配置。
  - 每个配置包含：名称、描述、启用开关、调度计划、存储目标、数据格式、过滤条件。
  - API 端点：`GET/POST /api/log-export-configs`、`GET/PUT/DELETE /api/log-export-configs/{id}`、`POST /api/log-export-configs/{id}/run`（立即触发）。

- **定时导出调度器 (ExportScheduler)**
  - 支持三种调度频率：每日 (daily)、每周 (weekly)、每月 (monthly)。
  - 可配置执行时间 (HH:MM)、星期几/日期、时区。
  - 后台每 60 秒检查一次，自动触发到期配置的导出任务。
  - 执行完成后自动更新 `last_run_at`、`last_run_status`、`next_run_at`。
  - 实现文件：`transports/bifrost-http/enterprise/export_scheduler.go`

- **云存储目标 (Export Destinations)**
  - **Amazon S3**：完整实现（bucket、region、prefix、credentials），使用 AWS SDK v2。
  - **Google Cloud Storage**：接口预留（需要额外 GCS SDK 依赖）。
  - **Azure Blob Storage**：接口预留（需要额外 Azure SDK 依赖）。
  - **Local Disk**：保持原有本地文件存储行为。
  - 路径模板支持：`{year}`、`{month}`、`{day}` 变量自动替换。
  - 实现文件：`transports/bifrost-http/enterprise/export_destinations.go`

- **前端 UI 全面重设计**
  - **导出配置管理区**：配置列表表格（Name、Destination、Schedule、Format、Last/Next Run、Status、Actions）+ 创建/编辑 Sheet。
  - **配置 Sheet 表单**：
    - General：名称、描述、启用开关、数据范围（LLM Logs / MCP Logs）
    - Schedule：频率（Daily/Weekly/Monthly）、时间、星期/日期、时区
    - Destination：类型选择（Local / Amazon S3 / Google Cloud Storage / Azure Blob Storage）+ 各类型专属配置字段（bucket、region、prefix、credentials）
    - Data：格式（JSONL/CSV）、压缩（None/Gzip）、最大行数
  - **一次性导出区**：保留原有手动导出功能（时间窗口 + 范围 + 格式 + 压缩 + 最大行数）。
  - **导出历史区**：任务列表（ID、Scope、Format、Status、Rows、Node、Created、Completed、Download）。
  - 操作按钮：Run Now（立即触发）、Edit（编辑配置）、Delete（删除配置）。

- **集群同步**
  - 新增 `ClusterConfigScopeLogExportConfig` 作用域。
  - 导出配置变更通过集群传播自动同步到所有节点。
  - 调度器在每个节点独立运行，通过 DB 状态避免重复执行。

- **验证通过**
  - `go build ./bifrost-http/...` ✓
  - `go test ./bifrost-http/handlers` ✓
  - `go test ./bifrost-http/enterprise` ✓
  - `npx tsc --noEmit` ✓

### 2026-04-12 | Base Commit 8090a43 | 日志导出下载修复 + 时间窗口支持

- **日志导出 (Log Exports) 下载 500 错误修复**
  - 根因：`NewLogExportService` 使用相对路径（`./bifrost-data/exports`）存储导出文件，导出任务的 `file_path` 存储的是相对路径。当应用工作目录变化或重启后，`os.Open(relativePath)` 找不到文件，返回 500 Internal Server Error。
  - 修复：在 `NewLogExportService` 中将 `baseDir` 和 `StoragePath` 规范化为绝对路径（`filepath.Abs`），确保导出文件路径始终是绝对路径。
  - 修改文件：`transports/bifrost-http/enterprise/exports.go`

- **日志导出新增时间窗口 (Time Window) 过滤**
  - 在导出表单中新增 Start Time 和 End Time 日期时间选择器。
  - 导出请求通过 `log_filters.start_time` / `log_filters.end_time` 传递给后端 `SearchFilters`，后端已支持按时间范围过滤（`timestamp >= start_time AND timestamp <= end_time`）。
  - 前端类型 `CreateLogExportRequest` 新增 `log_filters` 字段，包含 `start_time`、`end_time`、`providers`、`models`、`status`、`virtual_key_ids` 等过滤条件。
  - 修改文件：`ui/app/workspace/log-exports/page.tsx`、`ui/lib/types/enterprise.ts`

### 2026-04-12 | Base Commit b388fea | 护栏系统 + RBAC + 日志导出修复 + MCP 工具组增强

- **日志导出 (Log Exports) 服务自动启用修复**
  - 原因：LogExportService 需要在配置文件中显式设置 `log_exports.enabled: true` 才能启用，导致页面显示 "Log export service is not enabled"。
  - 修复：当日志插件 (LoggerPlugin) 可用且数据库已连接时，自动创建默认 LogExportsConfig 并启用服务。
  - 修改文件：`transports/bifrost-http/server/server.go`

- **护栏系统 (Guardrails) 完整实现**
  - 这是全新功能，之前仅有 "Contact Us" 占位页面。
  - **数据库 Schema**：
    - `TableGuardrailProvider`：护栏服务提供者配置（id, name, provider_type, enabled, timeout_seconds, config JSON）
    - `TableGuardrailRule`：护栏规则（id, name, description, enabled, apply_on, profile_ids, sampling_rate, timeout_seconds, cel_expression, scope/scope_id, priority）
    - 支持的提供者类型：`bedrock`（AWS Bedrock Guardrails）、`azure_content_moderation`（Azure 内容安全）、`patronus`（Patronus AI）、`mistral_moderation`（Mistral 审核）
    - 规则应用模式：`input`（仅输入）、`output`（仅输出）、`both`（双向）
  - **API 端点**：
    - `GET/POST /api/guardrails/providers` — 护栏提供者列表/创建
    - `GET/PUT/DELETE /api/guardrails/providers/{id}` — 护栏提供者详情/更新/删除
    - `GET/POST /api/guardrails/rules` — 护栏规则列表/创建
    - `GET/PUT/DELETE /api/guardrails/rules/{id}` — 护栏规则详情/更新/删除
  - **集群同步**：
    - 新增 `ClusterConfigScopeGuardrailProvider` 和 `ClusterConfigScopeGuardrailRule` 作用域
    - 在 `cluster_config_reload.go`、`cluster_config_propagation.go`、`cluster_governance_apply.go` 中完整实现跨节点同步
    - WebSocket 标签推送确保前端页面自动刷新
  - **前端 UI — 护栏规则配置页面** (`guardrailsConfigurationView.tsx`)：
    - 规则列表表格：Rule Name、Description、Apply On、Sampling Rate、Status、Actions
    - 规则创建/编辑 Sheet：Rule Name、Description、Enable Toggle、Apply On（Input Only / Output Only / Both 三选一）、Guardrail Profiles（多选下拉，关联已配置的提供者）、Sampling Rate (%)、Timeout (s)、CEL Rule Builder（复用 Routing Rules 的 CELRuleBuilder）、CEL Expression Preview
    - 删除确认对话框
  - **前端 UI — 护栏提供者配置页面** (`guardrailsProviderView.tsx`)：
    - 左侧边栏：四种提供者类型（AWS Bedrock、Azure Content Moderation、Patronus AI、Mistral Moderation），带图标和数量徽章
    - 主区域：所选提供者类型的配置列表（ID、Name、Is Enabled、Timeout、Actions）
    - 提供者创建/编辑 Sheet，每种类型有专属配置字段：
      - Bedrock：guardrail_identifier、guardrail_version、region、access_key（EnvVar）、secret_key（EnvVar）
      - Azure：endpoint、api_key（EnvVar）、categories
      - Patronus：api_key（EnvVar）、evaluator_id
      - Mistral：api_key（EnvVar）、model
  - 修改文件：`framework/configstore/tables/guardrails.go`（新建）、`framework/configstore/rdb.go`、`framework/configstore/store.go`、`framework/configstore/migrations.go`、`transports/bifrost-http/handlers/guardrails.go`（新建）、`transports/bifrost-http/handlers/cluster_config_reload.go`、`transports/bifrost-http/server/cluster_config_propagation.go`、`transports/bifrost-http/server/cluster_governance_apply.go`、`transports/bifrost-http/server/server.go`

- **RBAC 角色权限管理完整实现**
  - 之前仅有 "Contact Us" 占位页面。
  - **数据库 Schema**：
    - `TableRbacRole`：角色（id, name, description, is_default, is_system）
    - `TableRbacPermission`：权限分配（role_id, resource, operation）
    - `TableRbacUserRole`：用户角色映射（user_id, role_id）
  - **默认系统角色种子数据**：
    - `Admin`：全部资源的全部操作权限
    - `Developer`：除 Settings、RBAC、AuditLogs、UserProvisioning 写操作外的全部权限
    - `Viewer`：所有资源的 Read/View 只读权限
  - **覆盖 23 种资源**：GuardrailsConfig、GuardrailsProviders、GuardrailRules、UserProvisioning、Cluster、Settings、Users、Logs、Observability、VirtualKeys、ModelProvider、Plugins、MCPGateway、AdaptiveRouter、AuditLogs、Customers、Teams、RBAC、Governance、RoutingRules、PIIRedactor、PromptRepository、PromptDeploymentStrategy
  - **6 种操作**：Read、View、Create、Update、Delete、Download
  - **API 端点**：
    - `GET/POST /api/rbac/roles` — 角色列表/创建
    - `GET/PUT/DELETE /api/rbac/roles/{id}` — 角色详情/更新/删除（系统角色不可删除）
    - `GET /api/rbac/users` — 用户角色映射列表
    - `PUT /api/rbac/users/{user_id}` — 分配角色给用户
    - `GET /api/rbac/resources` — 获取所有可用资源和操作
    - `GET /api/rbac/check` — 权限检查
  - **前端 UI** (`rbacView.tsx`)：
    - 角色列表表格：Name、Description、Type（System/Custom badge）、Permissions Count、Actions
    - 创建/编辑角色对话框：Name、Description、权限矩阵（行=资源，列=操作，复选框）
    - 系统角色显示为只读
    - 删除确认对话框（系统角色不可删除）
  - 修改文件：`framework/configstore/tables/rbac.go`（新建）、`framework/configstore/rdb.go`、`framework/configstore/store.go`、`framework/configstore/migrations.go`、`transports/bifrost-http/handlers/rbac.go`（新建）、`transports/bifrost-http/server/server.go`

- **MCP 工具组 (Tool Groups) 增强**
  - 替换原有 "Contact Us" 占位页面为完整功能实现。
  - 汇总统计：Connected Servers / Total Tools / Enabled for Execution
  - 工具搜索：按名称或描述过滤
  - 服务器分组折叠面板：每个 MCP 服务器独立展开，显示其所有工具
  - 工具管理表格：Tool Name、Description、Execute（复选框）、Auto Execute（复选框）
  - 批量操作：Enable All / Disable All 按钮
  - 直接调用现有 MCP Client API 更新 `tools_to_execute` 和 `tools_to_auto_execute` 配置
  - 修改文件：`ui/app/_fallbacks/enterprise/components/mcp-tool-groups/mcpToolGroups.tsx`

- **验证通过**
  - `go build ./bifrost-http/...` ✓
  - `go test ./bifrost-http/handlers` ✓
  - `go test ./bifrost-http/loadbalancer` ✓
  - `go test ./bifrost-http/enterprise` ✓
  - `go test ./bifrost-http/lib` ✓
  - `go test ./configstore ./configstore/tables` (framework) ✓
  - `npx tsc --noEmit` ✓

### 2026-04-12 | Base Commit fa39857 | Adaptive Routing 按规则配置重构 + 日志导出 UI + 状态面板重设计

- **Adaptive Routing 配置从全局迁移到 Provider Routing Rules**
  - 新增 `rule_type` 字段（`direct` / `adaptive`），支持在 Routing Rules 中创建自适应负载均衡规则。
  - 新增 `adaptive_config` JSON 字段，承载每条规则的自适应配置（enabled、key_balancing、direction_routing、provider/model allowlist、tracker tuning）。
  - `TableRoutingRule` schema 扩展：`RuleType` + `AdaptiveConfig` 列，含 GORM BeforeSave/AfterFind 钩子自动序列化。
  - DB migration `add_routing_rule_adaptive_columns` 自动添加新列并回填已有规则为 `direct` 类型。
  - `config.schema.json` 已同步更新 routing_rule 定义，包含 `rule_type`、`adaptive_config` schema。

- **后端：自适应规则聚合机制**
  - 新增 `enterprise.AggregateAdaptiveRules()` 函数，扫描所有 enabled 且 rule_type="adaptive" 的规则，合并生成统一 `LoadBalancerConfig`。
  - 新增 `server.ReloadLoadBalancerFromAdaptiveRules()`，在 routing rule CRUD（create/update/delete）后自动触发聚合与热更新。
  - 支持 by-provider / by-key 作用域：规则的 `scope`（global/team/customer/virtual_key）控制适用范围，`adaptive_config.provider_allowlist` / `model_allowlist` 控制哪些 provider/model 启用自适应。
  - 集群同步：routing rule 变更通过已有的 `ClusterConfigScopeRoutingRule` 自动传播到所有节点，`adaptive_config` 作为 rule 的一部分随 JSON 一起同步。

- **Adaptive Routing 状态面板重设计**
  - 移除了原有的全局配置面板（Adaptive Routing Policy 区域）。
  - 新增 Live Metrics 仪表盘：Total Requests、Success Rate。
  - 新增 Total Traffic Distribution 表：按 key/provider/model 显示流量占比和进度条。
  - Direction Weights & Performance 表：Provider、Model、Weight、Success Rate、Errors、U/E/L Penalty、Health Status。
  - Route Weights & Performance 表：Key、Provider、Model、Weight、Success Rate、Errors、U/E/L Penalty、Momentum。
  - 所有表格支持 provider/model 筛选，5s 自动轮询刷新，集群聚合模式。

- **Routing Rules UI 扩展**
  - Sheet 表单新增 "Rule Type" 选择器（Direct / Adaptive）。
  - 选择 Adaptive 后展开自适应配置区：Enable、Key Balancing、Direction Routing、VK Direction Routing 开关 + Provider/Model Allowlist。
  - Routing Rules 表格新增 "Type" 列，显示 Direct / Adaptive badge。
  - Adaptive 规则的 targets 验证放宽（不强制要求 targets，允许纯 allowlist 作用域模式）。

- **日志导出（Log Exports）功能完善**
  - 后端已有完整 LogExportService（JSONL/CSV + gzip）、API（POST/GET /api/logs/exports、/api/mcp-logs/exports、/api/log-exports/{id}/download）和集群聚合。
  - 本轮新增 Log Exports UI 页面（`/workspace/log-exports`）：
    - 创建导出表单：Scope（LLM/MCP）、Format（JSONL/CSV）、Compression（None/Gzip）、Max Rows。
    - 导出历史表格：ID、Scope、Format、Status、Rows、Node、Created/Completed、Download 按钮。
    - 10s 自动轮询刷新，支持集群聚合查看所有节点的导出任务。
  - Sidebar 新增 "Log Exports" 导航入口。

- **验证通过**
  - `go test ./bifrost-http/loadbalancer` ✓
  - `go test ./bifrost-http/handlers` ✓
  - `go test ./bifrost-http/enterprise` ✓
  - `go test ./bifrost-http/lib` ✓
  - `go test ./configstore ./configstore/tables` (framework) ✓
  - `npx tsc --noEmit` ✓

### 2026-04-12 09:55:48 CST | Base Commit 90fbe5f7c | Adaptive Load Balancing 官网设计对齐增强

- 本轮重新对照了官网 `Adaptive Load Balancing` / `Provider Routing` 设计，并基于当前企业版 fork 的现有实现做了“最小破坏式”增强。
  - 没有去重写推理主链，也没有改动现有 cluster / governance / vault / audit 的核心执行路径。
  - 增强全部收口在现有 `transports/bifrost-http/loadbalancer` 插件、企业状态接口和自适应路由页面。

- 自适应路由核心能力从“即时按请求算分”升级为“异步预计算 + 状态机”。
  - 为 route（provider/model/key）和 direction（provider/model）新增了预计算 profile：
    - `state`
    - `score`
    - `weight`
    - `actual_traffic_share`
    - `expected_traffic_share`
  - 增加了 4 种健康状态：
    - `healthy`
    - `degraded`
    - `failed`
    - `recovering`
  - 增加了后台异步重计算循环，默认每 `5s` 重算一次 adaptive 权重与状态，而不是在热路径中每次请求都完整扫描。

- 补齐了官网设计里最关键的两层负载均衡能力。
  - `Level 1 - Direction`
    - `loadbalancer` 现在同时实现了 `HTTPTransportPlugin`。
    - 当请求体里的 `model` 仍是裸模型名、且 governance/routing rule 没有提前固定 provider 时，插件会在 HTTPTransportPreHook 阶段：
      - 根据 Model Catalog 找出可用 provider
      - 基于 direction profile 选择当前最优 provider
      - 自动生成 fallback provider 列表
      - 将请求体改写成后续 handler 可继续处理的 `provider/model` 形式
    - 这样保持了与当前 handler 体系兼容，不需要大范围改写请求解析器。
  - `Level 2 - Route`
    - key 级选择不再只依赖即时 EWMA，而是优先读取预计算 route profile。
    - 预计算权重范围收口到 `1-1000`。
    - 对 failed 路由启用 circuit-breaker 风格的 `0` 权重，并在探索流量路径中保留最小探测流量。

- 对齐官网设计补上了探索与恢复机制。
  - `exploration_ratio` 默认提升到 `25%`。
  - 增加 `recovery_half_life_seconds` 概念，用于在 recovering 状态下逐步降低惩罚。
  - 引入 `failed_error_threshold`、`degraded_error_threshold`、`failed_consecutive_failures` 等阈值，避免所有差路由都被同一种线性权重处理。

- 配置面也同步增强。
  - `load_balancer_config.tracker_config` 新增：
    - `recompute_interval_seconds`
    - `degraded_error_threshold`
    - `failed_error_threshold`
    - `failed_consecutive_failures`
    - `recovery_half_life_seconds`
    - `weight_floor`
    - `weight_ceiling`
  - `transports/config.schema.json` 已同步补齐 schema 定义。

- 企业状态接口和前端页面已对齐新指标。
  - `/api/adaptive-routing/status` 现在会返回 route / direction 的：
    - `state`
    - `score`
    - `weight`
    - `actual_traffic_share`
    - `expected_traffic_share`
  - Adaptive Routing 页面新增：
    - 状态列
    - 预计算权重列
    - 实际/期望流量占比列
  - 这样 Dashboard 上可以直接看到 route 是否 degraded/failed/recovering，以及 adaptive 权重是否已收敛。

- 稳定性与兼容性说明。
  - 本轮没有改动已有 provider API 调用路径。
  - 也没有改动 governance 已经负责的 routing rule / virtual key provider selection 语义。
  - 当 governance 已经给请求固定 provider，adaptive direction 级选择会自动跳过，避免双重改写。
  - 当请求已经显式带了 `fallbacks`，adaptive 不会强行覆盖，优先尊重显式配置。

- 本轮验证通过：
  - `go test ./transports/bifrost-http/loadbalancer`
  - `go test ./transports/bifrost-http/server ./transports/bifrost-http/handlers ./transports/bifrost-http/enterprise`
  - `go test ./transports/bifrost-http/lib -run TestConfigDataUnmarshalEnterpriseFields`
  - `go test ./transports/bifrost-http/integrations ./transports/bifrost-http/websocket`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
  - `npm exec next build -- --no-lint`
  - `npm exec tsc -- --noEmit`

### 2026-04-12 11:11:19 CST | Base Commit 820fdc3a4 | Adaptive Routing 配置面板与集群安全作用域收口

- 本轮把 Adaptive Routing 从“只有观测页”补成了“可配置 + 可集群同步 + 默认更安全”的完整闭环。
  - 新增了 Adaptive Routing 页面上的配置面板。
  - 后端 `/api/config` 已接入 `load_balancer_config` 的读取、保存与热更新。
  - Cluster config sync / drift fingerprint / controlled self-heal 也已经把 adaptive routing 纳入一致性范围。

- 为了适配企业内网私有化模型、按 provider / virtual key / team 管理的场景，本轮刻意把 Adaptive Routing 的默认行为收紧成“保守模式”。
  - `enabled = false`
  - `key_balancing_enabled = true`
  - `direction_routing_enabled = false`
  - `direction_routing_for_virtual_keys = false`
  - 这意味着默认只会在“同 provider / 同 model 的多个 key”之间做自适应 key balancing。
  - 不会默认对裸模型请求做全局 provider 改写。
  - 也不会默认对 virtual key 治理流量做跨 provider 重选。

- 新增了更细粒度的 Adaptive Routing 作用域控制项：
  - `key_balancing_enabled`
  - `direction_routing_enabled`
  - `direction_routing_for_virtual_keys`
  - `provider_allowlist`
  - `model_allowlist`
  - 这样可以把 Adaptive Routing 限定在指定 provider / model 上启用，而不是一刀切全局生效。

- 热更新路径也做了“最小破坏式”处理，避免影响现有运行中的插件链路。
  - 不再通过替换整条插件链来切换 Adaptive Routing 配置。
  - 而是始终保留同一个 built-in `loadbalancer` 插件实例，并在实例内原地更新 runtime policy。
  - 这样可以避免：
    - 现有 `KeySelector` 指针失效
    - 状态接口拿到旧插件实例
    - 集群节点保存配置后本地没真正热生效
  - route / direction 的历史指标会在配置热更新时复制到新的 tracker，不会因为改参数把当前统计全部清空。

- 前端 Adaptive Routing 页面已补充可直接操作的配置项：
  - 总开关
  - Key Balancing 开关
  - Provider Direction Routing 开关
  - Virtual Key 流量是否允许 Direction Routing
  - Provider / Model allowlist
  - 关键 tracker tuning 参数
  - 页面文案也明确提示了企业部署下的推荐配置：默认开启 key balancing、默认关闭 provider direction routing。

- 集群一致性方面，本轮已经补齐：
  - `ClusterConfigScopeLoadBalancer`
  - ConfigStore 持久化键：`load_balancer_config`
  - cluster fanout 后 peer 自动热更新
  - cluster config drift 指纹中新增 `adaptive_routing`
  - controlled self-heal 在检测到 `adaptive_routing` drift 时会低频触发从 ConfigStore 自愈 reload
  - Web UI 保存配置后也会触发 `Config / AdaptiveRouting / ClusterNodes` 的前端缓存失效，避免页面停留在旧状态

- 本轮新增回归覆盖：
  - `/api/config` 更新 Adaptive Routing 配置后，会：
    - 落库
    - 调用 runtime reload
    - 广播 cluster config change
  - Adaptive Routing 插件热更新后：
    - 历史 route / direction 指标仍保留
    - 关闭 direction routing 后，裸模型请求不会再被自动改写 provider

- 本轮验证通过：
  - `go test ./transports/bifrost-http/loadbalancer`
  - `go test ./transports/bifrost-http/handlers`
  - `go test ./transports/bifrost-http/server`
  - `go test ./transports/bifrost-http/lib -run TestConfigDataUnmarshalEnterpriseFields`
  - `go test ./transports/bifrost-http/integrations`
  - `go test ./transports/bifrost-http/websocket`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
  - `npm exec tsc -- --noEmit`
  - `npm exec next build -- --no-lint`

### 2026-04-12 | Commit 628940d | Adaptive Load Balancing 架构重设计：从路由规则解耦为全局配置

> **核心变更**：Adaptive Load Balancing 从错误的"路由规则内嵌"架构重设计为正确的"全局系统级配置"架构，对齐官方文档的两层自适应设计（Direction + Route）。

- **问题诊断**

  此前的实现将 Adaptive Load Balancing 作为 Routing Rules 的一种 `rule_type="adaptive"` 嵌入，带来以下架构问题：
  1. 选择 Adaptive 规则后，Provider Allowlist / Model Allowlist 的作用不明确——到底是匹配路由还是限定自适应范围？
  2. 自适应规则的 targets 和 routing rule 的 target provider 产生冲突——配了 adaptive rule 后外层还需要配 target provider 吗？
  3. 多条 adaptive 规则的配置需要通过 `AggregateAdaptiveRules()` 合并，逻辑复杂且容易不一致。
  4. 集群同步将 adaptive config 作为 routing rule 的一部分传播（`ClusterConfigScopeRoutingRule`），而非独立配置范围。
  5. 与官方文档设计不符——官方设计中 Adaptive LB 是自动作用于所有已配置 provider/key 的全局系统功能，不是路由规则概念。

- **后端架构重设计（Go）**

  - **移除路由规则中的自适应字段**：
    - 从 `TableRoutingRule` 结构体中移除 `RuleType`、`AdaptiveConfig`、`ParsedAdaptiveConfig` 字段
    - 路由规则回归纯粹的请求路由职责——只有 Direct（静态加权路由）模式
    - 移除 BeforeSave/AfterFind 中的 adaptive config 序列化/反序列化逻辑
    - DB migration `add_routing_rule_adaptive_columns` 标记为 legacy no-op，保持已有数据库兼容

  - **移除规则聚合机制**：
    - 从 `enterprise/load_balancer_config.go` 中移除 `RuleAdaptiveConfig` 结构体
    - 移除 `AggregateAdaptiveRules()` 函数（原来扫描所有 adaptive 规则合并为单一配置）
    - 从 `server/load_balancer_config.go` 中移除 `ReloadLoadBalancerFromAdaptiveRules()` 函数
    - 从 `server.go` 的 `ReloadRoutingRule()` 和 `RemoveRoutingRule()` 中移除自适应规则重聚合逻辑

  - **移除 API 中的自适应规则类型**：
    - `CreateRoutingRuleRequest` / `UpdateRoutingRuleRequest` 移除 `rule_type` 和 `adaptive_config` 字段
    - 创建路由规则时不再区分 direct/adaptive，统一要求 targets（加权路由目标）
    - 更新路由规则时移除 rule_type 切换和 adaptive_config 更新逻辑

  - **新增独立的 Adaptive Routing Config API**：
    - `GET /api/adaptive-routing/config` — 获取当前全局自适应路由配置
    - `PUT /api/adaptive-routing/config` — 更新全局自适应路由配置（持久化 + 热更新 + 集群传播）
    - 新增 `LoadBalancerConfigManager` 接口，提供 `GetLoadBalancerConfig()` / `ReloadLoadBalancerConfig()` / `ApplyClusterLoadBalancerConfig()` 方法
    - `EnterpriseHandler` 扩展：增加 `lbConfig LoadBalancerConfigManager` 和 `propagate ClusterConfigPropagator` 依赖

  - **集群同步修复**：
    - 配置更新通过 `ClusterConfigScopeLoadBalancer` 独立传播，不再与路由规则混淆
    - 更新 API 保存后自动调用 `PropagateClusterConfigChange()`，确保所有节点实时同步
    - 配置持久化在 `governance_config` 表的 `load_balancer_config` 键下

- **前端架构重设计（React/TypeScript）**

  - **路由规则表单简化**：
    - 移除 "Rule Type" 选择器（Direct / Adaptive 下拉框）
    - 移除 Adaptive 配置面板（Enable、Key Balancing、Direction Routing、VK Direction Routing 开关 + Provider/Model Allowlist）
    - 路由规则表格移除 "Type" 列
    - 表单验证简化：所有规则统一要求 targets，weights 必须求和为 1
    - 修改文件：`routingRuleSheet.tsx`、`routingRulesTable.tsx`

  - **Adaptive Routing 页面新增配置面板**：
    - 页面标题从 "Live Metrics" 更名为 "Adaptive Load Balancing"，反映其作为配置+监控的统一入口
    - 页面顶部新增 Configuration 卡片，包含：
      - **4 个功能开关**（2×2 网格）：Enable（总开关）、Key Balancing、Direction Routing、VK Direction Routing
      - **Provider / Model Allowlist**：文本域，逗号分隔，空值表示全部
      - **Advanced Tracker Settings**：可折叠区域，12 个高级调参项（EWMA Alpha、Error/Latency/Consecutive Failure Penalty、Minimum Samples、Exploration/Jitter Ratio、Recompute Interval、Degraded/Failed Error Threshold、Weight Floor/Ceiling）
      - **Unsaved Changes 提示**：实时对比本地编辑与服务端配置，有变更时显示 amber 警告
      - **Save 按钮**：一键保存并传播到集群所有节点
    - 提示信息更新：未启用时提示"Enable adaptive routing in the configuration panel above"而非"Create an adaptive routing rule"
    - 修改文件：`ui/app/workspace/adaptive-routing/page.tsx`

  - **TypeScript 类型重构**：
    - 新增 `ui/lib/types/adaptiveRouting.ts`：独立的 `AdaptiveRoutingConfig`、`AdaptiveRoutingTrackerConfig`、`DEFAULT_ADAPTIVE_ROUTING_CONFIG` 类型定义
    - 从 `ui/lib/types/routingRules.ts` 移除：`RoutingRuleType`、`AdaptiveConfig`、`DEFAULT_ADAPTIVE_CONFIG` 及所有 adaptive 相关字段
    - `RoutingRule`、`CreateRoutingRuleRequest`、`RoutingRuleFormData` 等接口中移除 `rule_type` 和 `adaptive_config`

  - **RTK Query API 扩展**：
    - `enterpriseApi` 新增 `getAdaptiveRoutingConfig` query endpoint（`GET /api/adaptive-routing/config`）
    - `enterpriseApi` 新增 `updateAdaptiveRoutingConfig` mutation endpoint（`PUT /api/adaptive-routing/config`）
    - 导出 `useGetAdaptiveRoutingConfigQuery` / `useUpdateAdaptiveRoutingConfigMutation` hooks
    - 缓存标签：`AdaptiveRouting`，更新后自动失效状态和配置查询

- **配置数据流对比**

  ```
  旧架构（已移除）：
  Routing Rule (rule_type=adaptive, adaptive_config={...})
    → AggregateAdaptiveRules() 扫描所有规则
    → 合并为 LoadBalancerConfig
    → 更新 loadbalancer 插件
    → 集群通过 ClusterConfigScopeRoutingRule 传播

  新架构：
  PUT /api/adaptive-routing/config
    → 持久化到 governance_config 表 (key=load_balancer_config)
    → ReloadLoadBalancerConfig() 热更新插件
    → ClusterConfigScopeLoadBalancer 传播到所有节点
  ```

- **修改文件清单**

  | 文件 | 变更说明 |
  |------|----------|
  | `framework/configstore/tables/routing_rules.go` | 移除 RuleType、AdaptiveConfig 字段 |
  | `framework/configstore/migrations.go` | 标记 adaptive columns migration 为 legacy no-op |
  | `transports/bifrost-http/enterprise/load_balancer_config.go` | 移除 RuleAdaptiveConfig、AggregateAdaptiveRules |
  | `transports/bifrost-http/server/load_balancer_config.go` | 移除 ReloadLoadBalancerFromAdaptiveRules，新增 GetLoadBalancerConfig |
  | `transports/bifrost-http/server/server.go` | 移除规则聚合调用，传入新接口参数 |
  | `transports/bifrost-http/handlers/enterprise.go` | 新增 LoadBalancerConfigManager 接口、GET/PUT config 端点 |
  | `transports/bifrost-http/handlers/governance.go` | 移除 adaptive 相关请求/响应字段和验证 |
  | `transports/bifrost-http/handlers/enterprise_test.go` | 更新 NewEnterpriseHandler 调用签名 |
  | `transports/bifrost-http/server/cluster_config_propagation_test.go` | 更新 NewEnterpriseHandler 调用签名 |
  | `ui/app/workspace/adaptive-routing/page.tsx` | 新增 Configuration 面板组件 |
  | `ui/app/workspace/routing-rules/views/routingRuleSheet.tsx` | 移除 adaptive 规则类型和配置面板 |
  | `ui/app/workspace/routing-rules/views/routingRulesTable.tsx` | 移除 Type 列 |
  | `ui/lib/types/adaptiveRouting.ts` | 新建：独立的 adaptive routing 类型 |
  | `ui/lib/types/routingRules.ts` | 移除 adaptive 相关类型 |
  | `ui/lib/store/apis/enterpriseApi.ts` | 新增 config query/mutation endpoints |

- **验证通过**
  - `go build ./...`（framework + transports）✓
  - `go test ./bifrost-http/handlers` ✓
  - `go test ./bifrost-http/loadbalancer` ✓
  - `npx tsc --noEmit` ✓

### 2026-04-12 | Working Tree | Adaptive Load Balancing 二次纠偏：回归 Provider Routing Rules，并补齐集群统计同步

> **说明**：这一轮对上面 `Commit 628940d` 的 Adaptive 架构做了纠偏。实践中发现将 Adaptive 完全做成“全局配置入口”会带来明显的语义混乱，和官网 `Adaptive Load Balancing / Provider Routing` 的两层设计也不一致。因此本轮将 **策略入口重新收回到 Provider Routing Rules**，而全局 `load_balancer_config` 仅保留为 **算法默认值 / engine defaults**。

- **架构纠偏**

  - 恢复 `Routing Rule` 的 `rule_type = direct | adaptive`
  - 恢复 `adaptive_config` 到 routing rule 数据模型，并重新打通：
    - DB 表结构
    - migration 修复
    - governance handler 的 CRUD
    - governance routing engine 的匹配输出
    - cluster routing rule 同步载荷
  - `direct` 与 `adaptive` 的职责重新分离：
    - `direct`：显式 target provider/model/key + 静态权重/fallback
    - `adaptive`：声明候选 provider/model 集合，由 direction-level adaptive 选择 provider，再由 route-level adaptive 选择 key
  - `adaptive` 规则下不再允许 `key_id` 绑定，避免把 provider-level 和 key-level 两层策略混在一起

- **Adaptive 运行时逻辑修正**

  - Governance 命中 `adaptive` 规则后，不再直接把请求改写成固定 provider/model
  - 改为把规则级 adaptive 策略写入上下文：
    - `BifrostContextKeyGovernanceAdaptiveRoutingConfig`
    - `BifrostContextKeyGovernanceAdaptiveRoutingTargets`
    - `BifrostContextKeyGovernanceAdaptiveRoutingFallbacks`
  - HTTP transport adaptive plugin 读取这些上下文信息后执行：
    - **Direction-level 选择**：在规则声明的候选 provider/model 集合中选择最佳方向
    - **Route-level 选择**：在已选 provider 的 key 池内继续做自适应 key 选择
  - 这样 `Provider Allowlist / Model Allowlist` 不再承担“业务规则配置”的职责，只保留全局默认行为过滤的意义

- **集群 Adaptive 真正生效**

  - 新增 `transports/bifrost-http/server/load_balancer_cluster_sync.go`
  - 每 5 秒通过 `/_cluster/adaptive-routing/status` 拉取 peer 的 **local-only 原始 adaptive 快照**
  - 本节点 tracker 合并 remote route/direction snapshots，并在异步预计算阶段统一计算权重
  - 这样 cluster 下 direction / route 的 score、weight、traffic share 不再是“各节点各算各的”
  - 同时保留双视角：
    - **public status**：返回 cluster-aware merged 视图
    - **internal peer endpoint**：只返回 local-only 原始快照，避免双重累计

- **Adaptive Dashboard 收口**

  - `Adaptive Routing` 页面现在回归为 **纯状态观测页**
  - 不再直接在状态页上编辑 Adaptive 配置
  - 页面结构重新对齐官方 dashboard 思路：
    - Live Metrics
    - Active Alerts
    - Total Traffic Distribution
    - Direction Weights & Performance
    - Route Weights & Performance
  - 页面默认读取本节点的 cluster-aware merged 状态，不再通过 `cluster=true` 简单拼表
  - 策略配置入口回到 `Routing Rules` 页面：
    - 新增 `Direct Rule / Adaptive Load Balancing` 规则类型切换
    - `Adaptive Policy` 表单块
    - `Adaptive Candidates` 候选 provider/model 配置
    - Routing Rules 表格新增 `Type` 展示

- **集群与状态接口稳定性**

  - internal adaptive status 接口改为使用 `ListLocalSnapshots / ListLocalDirectionSnapshots`
  - public adaptive status 接口使用 merged cluster-aware status
  - 增加 remote snapshot TTL 和 peer prune 逻辑，避免节点退出后旧统计长期滞留

- **本轮验证**

  - `go test ./transports/bifrost-http/loadbalancer ./transports/bifrost-http/handlers ./transports/bifrost-http/server ./plugins/governance`
  - `go test ./framework/configstore/...`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
  - `npm exec next build -- --no-lint`
  - `npm exec tsc -- --noEmit`

- **新增回归重点**

  - `adaptive` rule 命中后返回 rule-scoped adaptive decision，而不是直接 target rewrite
  - remote adaptive snapshots 会合并到 cluster-aware status，但不会污染 local-only internal peer status
  - internal `/_cluster/adaptive-routing/status` 永远只暴露本节点原始快照
