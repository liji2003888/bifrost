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
