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

### 2026-04-14 | Working Tree | 选择性回迁 `transports/v1.5.0-prerelease3` 第一批安全增量

> **说明**：这一轮没有整包合并上游 `prerelease3`，而是只回迁了对当前企业版 fork 收益高、且对现有 cluster / governance / vault / audit 定制影响最小的三组能力：`async user values` 透传、`model alias tracking`、以及 `streaming transport post-hook` 的安全收口。

- **Async User Values 透传**

  - `framework/logstore.AsyncJobExecutor` 的 `SubmitJob` 现在会携带当前请求的 user values 快照
  - 异步任务执行时会把这些值重新写回新的 `BifrostContext`
  - 这样 async 场景下：
    - governance / virtual key 相关上下文
    - request-scoped 自定义上下文
    - `BifrostIsAsyncRequest`
    都能在后台执行链里保持一致
  - 同时对传入 map 做了一层浅拷贝，避免 goroutine 与外层调用方共享同一个 map 引起状态漂移

- **Model Alias Tracking**

  - 为响应与错误的 extra fields 增加：
    - `original_model_requested`
    - `resolved_model_used`
  - 日志表 `logs` 新增 `alias` 列，用于单独保留“请求时写的模型别名”
  - 日志写入逻辑现在区分：
    - `model`：真正执行落到的 resolved model
    - `alias`：原始请求模型名 / alias
  - 这样在 provider routing / adaptive routing / model alias 场景下，更容易排查：
    - 用户请求的是什么
    - 最终实际跑到的是什么
  - UI 日志详情页也同步展示了 `Requested Model`

- **Streaming Transport Post-Hook 安全收口**

  - streaming 请求现在不再依赖流结束后仍然访问原始 `fasthttp.RequestCtx` 做 transport post-hook
  - 中间件会在响应写回前捕获 `HTTPRequest / HTTPResponse` 快照
  - stream 结束时再通过 deferred completer 安全执行 transport post-hook
  - 这一步主要是为了降低 streaming 场景下：
    - ctx 生命周期结束
    - trace completion 顺序
    - post-hook 读取已失效 transport 对象
    这类问题的风险

- **本轮验证**

  - `go test ./framework/logstore`
  - `go test ./plugins/logging`
  - `go test ./transports/bifrost-http/handlers`
  - `go test ./transports/bifrost-http/integrations`
  - `go test ./transports/bifrost-http/server`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
  - `npm exec next build -- --no-lint`
  - `npm exec tsc -- --noEmit`

- **说明**

  - `go test ./transports/bifrost-http/...` 本轮再次出现长时间无输出的现象，因此没有把它作为“整包通过”的结论依据
  - README 中记录的都是实际完成并确认通过的分层验证结果

### 2026-04-14 | Working Tree | MCP Auth Config 管理页落地 + OAuth MCP 管理链增强

> **说明**：这一轮把原本还是占位态的 `MCP Auth Config` 页面补成了真实可用的管理页，同时对齐了当前 fork 已经存在的 OAuth-backed MCP 后端能力。实现重点放在 **OAuth 配置管理、幂等撤销、集群状态同步**，没有去改动已有的 MCP tool execution / cluster / governance 主执行链。

- **MCP Auth Config 页面正式可用**

  - `ui/app/workspace/mcp-auth-config/page.tsx` 不再指向 enterprise 占位页
  - 新增真实管理页：
    - `ui/app/workspace/mcp-auth-config/views/mcpAuthConfigView.tsx`
  - 页面现在支持：
    - 查看 OAuth MCP auth configs 列表
    - 搜索与按状态筛选
    - 查看 linked MCP client / pending MCP client
    - 查看 token 到期时间与 scopes
    - 执行 `Complete OAuth`
    - 执行 `Cancel / Revoke`
    - 复制 `oauth_config_id`
  - 页面说明也明确了产品边界：
    - **新建** OAuth MCP Server 仍从 `MCP Registry` 发起
    - 当前页负责 **管理和完成** 已存在的 OAuth 配置

- **新增 OAuth Config 管理 API**

  - `GET /api/oauth/configs`
    - 支持 `limit / offset / search / status`
    - 返回 OAuth config 基础信息、token 元数据、linked MCP client / pending MCP client 摘要
  - 后端实现落在：
    - `transports/bifrost-http/handlers/oauth2.go`
    - `framework/configstore/store.go`
    - `framework/configstore/rdb.go`
  - 前端 RTK Query 已接入：
    - `ui/lib/store/apis/mcpApi.ts`
    - `ui/lib/types/mcp.ts`

- **OAuth 撤销语义补成幂等**

  - 之前 `DELETE /api/oauth/config/{id}` 在 `oauth_config.token_id == nil` 时会直接失败
  - 现在即使是：
    - `pending`
    - `failed`
    - `expired`
    - `authorized 但 token 缺失`
    的 OAuth config，也可以安全撤销
  - 撤销行为现在统一为：
    - 标记 `status = revoked`
    - 清空 `MCPClientConfigJSON`
    - 如果存在 token，则删除 token
  - 这样 `MCP Auth Config` 页面上的 `Cancel / Revoke` 在单节点和多节点下都具备一致语义
  - 关键实现：
    - `framework/oauth2/main.go`

- **集群同步保持可用**

  - OAuth config / token 的 cluster propagation 继续沿用现有 sync hook 与 cluster config change 机制
  - 本轮新增的管理动作没有绕开原有同步链
  - 也就是说：
    - 任一节点发起 revoke/cancel
    - OAuth config / token 状态会继续通过已有 cluster sync 传播
    - `MCP Auth Config` 页面轮询看到的是共享 ConfigStore + 集群同步后的状态

- **吸收 `prerelease3` 的 OAuth MCP 返回增强**

  - `POST /api/mcp/client` 在返回 `pending_oauth` 时，新增：
    - `status_url`
    - `complete_url`
    - `next_steps`
  - 这样前端或外部客户端在处理 OAuth MCP 创建流程时，不再只有 `authorize_url`，还会拿到后续完成授权所需的完整引导信息
  - 这部分是和 `transports/v1.5.0-prerelease3` 最贴近、也最安全的 MCP 增量回迁之一

- **当前边界说明**

  - 本轮补齐的是 **OAuth-backed MCP server 配置管理**
  - 还没有把官方 `MCP with Federated Auth` 文档中那条“把普通企业 API（Postman/OpenAPI/cURL/内置 UI）直接动态转换成 MCP tools”的完整 data plane 做完
  - 也就是说：
    - **MCP Auth Config 页面可用了**
    - **OAuth MCP server 管理链可用了**
    - 但“普通私有 API -> hosted MCP tools”的完整后端执行面仍然不是完整闭环

- **本轮验证**

  - `go test ./framework/oauth2`
  - `go test ./framework/configstore/...`
  - `go test ./transports/bifrost-http/handlers`
  - `go test ./transports/bifrost-http/server`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
  - `npm exec next build -- --no-lint`
  - `npm exec tsc -- --noEmit`

### 2026-04-14 23:59:51 CST | Base Commit 3eb9c7364 | 日志过滤、Model Catalog 时间范围与 Pricing 远端同步停用

- **LLM Logs：logging headers 过滤增强**

  - 后端 `GET /api/logs/filterdata` 现在会把 `client.logging_headers` 中配置的 header key 一并返回到 `metadata_keys`
  - 即使某个 logging header 目前还没有 distinct values，也会在前端过滤面板中显示出来，便于直接按 `header key + custom value` 做过滤
  - 这让后续按项目标识、用户标识、租户标识等自定义 header 统计日志成为可用路径
  - 关键实现：
    - `transports/bifrost-http/handlers/logging.go`
    - `ui/components/filters/filterPopover.tsx`

- **Logs Settings：`enable_logging=false` 改为“元数据日志模式”**

  - 之前 `enable_logging=false` 会让 logging plugin 整体不注册，导致连 token / latency / status / routing context 这类统计数据也没有
  - 现在调整为：
    - `enable_logging=true`：记录完整内容日志
    - `enable_logging=false`：仍记录运营与统计日志，但不记录输入/输出正文
  - 也就是说，以下数据在关闭 full content logs 后仍然会保留：
    - status
    - latency
    - token usage
    - routing / provider / key / virtual key 等上下文
    - configured logging headers
    - cost（若有）
  - `disable_content_logging=true` 与 `enable_logging=false` 现在都走统一的“正文关闭、统计保留”逻辑
  - logging plugin 也改成了 client config 热更新可重绑，不需要 restart
  - 关键实现：
    - `plugins/logging/main.go`
    - `plugins/logging/operations.go`
    - `transports/bifrost-http/server/plugins.go`
    - `transports/bifrost-http/handlers/config.go`
    - `ui/app/workspace/config/views/loggingView.tsx`
    - `ui/components/loggingDisabledView.tsx`

- **Model Catalog：支持任意时间范围 + 快速时间选择**

  - Model Catalog 页面不再固定只能看最近 24h / 30d
  - 现在支持：
    - `Last 24 hours`
    - `Last 7 days`
    - `Last 30 days`
    - 任意自定义时间范围
  - 页面上的 summary cards、provider traffic/cost 统计、models used 列表都会统一按当前所选时间范围查询与展示
  - 交互风格与 Logs 页的时间过滤器保持一致
  - 关键实现：
    - `ui/app/workspace/model-catalog/views/modelCatalogView.tsx`
    - `ui/app/workspace/model-catalog/views/modelCatalogTable.tsx`

- **Pricing Config：支持显式停用远端 pricing 拉取**

  - 为适配内网部署和“不需要成本计算”的场景，framework pricing 配置现在支持显式停用远端 datasheet 同步：
    - `pricing_url = ""`
    - 或 `pricing_sync_interval = 0`
  - 停用后行为为：
    - 不再执行初始 remote pricing datasheet 拉取
    - 不再执行后台定时 pricing sync
    - 不再执行 model-parameters 的远端同步
    - `Force Sync Now` 前端按钮会禁用
    - 已有本地 / DB pricing 数据仍可继续读取，不会强制清空
  - 这次是“停远端同步”，不是“删除 pricing 能力”，因此对现有运行时影响最小
  - 配置校验也同步放宽为允许 `pricing_sync_interval = 0`
  - 关键实现：
    - `framework/modelcatalog/main.go`
    - `framework/modelcatalog/sync.go`
    - `framework/modelcatalog/main_test.go`
    - `transports/bifrost-http/lib/config.go`
    - `transports/bifrost-http/lib/config_test.go`
    - `transports/bifrost-http/handlers/config.go`
    - `transports/config.schema.json`
    - `ui/app/workspace/config/views/pricingConfigView.tsx`

- **当前边界说明**

  - 这轮没有去改日志查询后端的 metadata filter 语义；原有 `metadata_<key>=<value>` 过滤能力本来就已经存在，这次主要补的是：
    - configured logging headers 在过滤器中可见
    - `enable_logging=false` 时日志插件仍然保留统计数据
  - Pricing 远端同步停用后，如果部署方本身没有本地 pricing 数据，cost 相关统计会为空或缺失；这是预期行为

- **本轮验证**

  - `go test ./plugins/logging`
  - `go test ./framework/modelcatalog`
  - `go test ./transports/bifrost-http/handlers`
  - `go test ./transports/bifrost-http/server`
- `go test ./transports/bifrost-http/lib -run 'TestResolveFrameworkPricingConfig|TestConfigDataUnmarshalEnterpriseFields|TestDoesNotExist'`
- `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
- `npm exec tsc -- --noEmit`
- `npm exec next build -- --no-lint`

---

## 2026-04-15 00:21:38 CST | Base Commit 3eb9c7364 | prerelease3 安全回迁（MCP discovered tools persistence）

- **本轮目标**

  - 在不影响现有集群高可用、Adaptive、Governance、Vault、Audit 主链路的前提下，继续吸收上游 `transports/v1.5.0-prerelease3` 的低风险增量能力。
  - 这轮优先回迁的是 MCP 相关的“已发现工具持久化”能力，避免 MCP client 短暂断连、节点切换或服务重启后，前端列表立即丢失工具视图。

- **本轮落地**

  - **MCP Client 新增“已发现工具 / 工具名映射”持久化**

    - 运行时在首次工具发现和后续 tool sync 成功后，会把：
      - `discovered_tools`
      - `tool_name_mapping`
      持久化进 ConfigStore
    - 这样即使 MCP client 当前未连接，或者页面读取的是分页查询路径，也能回退展示“最近一次成功发现到的工具”
    - 关键实现：
      - `core/mcp/mcp.go`
      - `core/mcp/clientmanager.go`
      - `core/mcp/toolsync.go`
      - `framework/configstore/tables/mcp.go`
      - `framework/configstore/rdb.go`
      - `framework/configstore/migrations.go`

  - **服务端新增 MCP discovered tools 持久化回调**

    - transport 层在初始化 MCPManager 时会注入 `PersistDiscoveredTools` 回调
    - 回调会同步完成三件事：
      - 更新内存态 `Config.MCPConfig`
      - 更新 ConfigStore 中对应 MCP client 行
      - 广播前端 `MCPClients` store_update，促使页面自动刷新
    - 这样不会改动推理主链，只增强 MCP 管理面和观测面
    - 关键实现：
      - `transports/bifrost-http/server/server.go`
      - `transports/bifrost-http/lib/config.go`
      - `transports/bifrost-http/server/cluster_config_propagation.go`

  - **MCP 列表页支持 disconnected 状态下展示最近发现工具**

    - MCP Registry 页面现在在运行时工具列表为空时，会自动回退到持久化的 discovered tools
    - 这样即使某个节点暂时未连上 MCP server，UI 仍能显示最近一次成功发现的工具数量和工具列表，不会一下子退化成空白
    - 关键实现：
      - `transports/bifrost-http/handlers/mcp.go`
      - `ui/app/workspace/mcp-registry/views/mcpClientsTable.tsx`

- **集群影响评估**

  - 这轮没有改动 LLM 推理主链、provider queue、adaptive 选择器、cluster config reload 主流程
  - 新增能力依赖：
    - 共享 ConfigStore
    - 现有 MCP client 配置同步机制
    - 现有前端 store_update 广播
  - 因此在多节点环境下，某节点发现到的新工具快照可以被持久化，其它节点读取 MCP 配置或分页列表时也能看到一致的最近状态
  - 这轮仍然没有去改动“普通企业 API 自动托管成 MCP tools”的数据平面，这部分后续继续按 MCP Federated Auth 的完整方案推进

- **本轮验证**

  - `go test ./mcp -run TestDoesNotExist`
  - `go test ./configstore -run 'TestMCPClientDiscoveredToolsPersistAcrossCreateAndUpdate|TestDoesNotExist'`
  - `go test ./bifrost-http/handlers -run 'TestAddMCPClientPropagatesClusterConfigChange|TestGetMCPClientsPaginatedFallsBackToPersistedDiscoveredTools'`
  - `go test ./bifrost-http/server -run 'TestApplyClusterConfigChangeMCPClientLifecycle|TestPersistMCPDiscoveredToolsUpdatesStoreAndRuntimeConfig'`
- `go test ./bifrost-http/server ./bifrost-http/handlers`
- `go test ./bifrost-http -tags embedui -run TestDoesNotExist`
- `npm exec tsc -- --noEmit`
- `npm exec next build -- --no-lint`

---

## 2026-04-15 08:13:49 CST | Base Commit 3eb9c7364 | prerelease3 安全回迁（OAuth token endpoint 重试与永久失败过期标记）

- **本轮目标**

  - 继续按 `transports/v1.5.0-prerelease3` 的稳定性增强路线推进，但仍然保持“企业版 cluster 主链不乱、推理热路径不动”的原则。
  - 这轮聚焦在 `framework/oauth2`，补齐 OAuth token endpoint 的重试机制，以及永久失败时的配置状态收敛。

- **本轮落地**

  - **OAuth token endpoint 新增受控重试**

    - `authorization_code` 和 `refresh_token` 两条 token 交换链路，都会统一走新的 `callTokenEndpoint(ctx, ...)`
    - 对以下场景做最多 3 次重试：
      - 网络抖动
      - 连接失败 / timeout
      - 5xx / 429 / 其它非 400、401 的临时失败
    - 重试采用指数退避，不会在最后一次失败后多等一轮
    - 关键实现：
      - `framework/oauth2/main.go`

  - **永久失败自动标记 OAuth Config 为 expired**

    - 当 refresh token 被 provider 明确拒绝时，会把 oauth config 状态标记为 `expired`
    - 当前按上游策略认定为“永久失败”的场景包括：
      - `401 unauthorized`
      - `400 invalid_grant`
      - `400 unauthorized_client`
    - 这样 MCP Auth Config 页面和多节点状态页不会一直卡在“authorized 但实际已不可刷新”的假状态
    - 本地实现还额外保留了已有的 sync hook，因此状态变化会继续通过 cluster sync 传播到其他节点

  - **瞬时失败不误伤配置状态**

    - 如果只是 provider 临时故障、网络波动或 5xx，OAuth config 仍保持原状态，不会被误标记成 `expired`
    - 这点对集群尤其重要，可以避免一个节点瞬时失败就把共享 OAuth 状态打坏

- **集群影响评估**

  - 这轮只改了 `framework/oauth2`，没有改 MCP server 连接主链、LLM 推理主链、cluster config fanout 主逻辑
  - 由于本地 OAuth provider 已经有 sync hook，这次新增的 `expired` 状态更新会沿用现有同步通道传播，不会变成单节点私有状态
  - 也就是说：
    - 瞬时失败：不改状态，集群保持稳定
    - 永久失败：统一标记为 `expired`，集群状态一致

- **本轮验证**

  - `go test ./oauth2`
  - `go test ./bifrost-http/handlers ./bifrost-http/server`
  - `go test ./bifrost-http -tags embedui -run TestDoesNotExist`

- **新增回归覆盖**

  - token endpoint 遇到瞬时失败会重试并最终成功
  - refresh token 遇到 `invalid_grant` 会把 oauth config 标记为 `expired`
  - refresh token 遇到 5xx 等临时故障时，不会误把 oauth config 标记成 `expired`

---

## 2026-04-15 08:54:33 CST | Base Commit 3eb9c7364 | 高基数 Metadata 过滤优化 + prerelease3 Azure Passthrough 回迁

- **本轮目标**

  - 解决 LLM Logs 里 `logging_headers` 元数据过滤在高基数字段下不可用的问题，尤其是 `userId`、`projectId` 这类十万级唯一值场景。
  - 继续选择性回迁 `transports/v1.5.0-prerelease3` 中风险较低、对现网能力友好的增量功能。

- **本轮落地**

  - **LLM Logs Metadata 过滤改成“按 Key 精确输入 Value”**

    - `GET /api/logs/filterdata` 不再枚举 metadata 的 distinct value，只返回可过滤的 metadata key 列表
    - 前端 Filter Popover 也不再把每个 metadata value 全量塞进勾选列表，而是改成：
      - 显示 `Metadata: <key>`
      - 提供一个精确值输入框
      - 输入值后按现有 `metadata_<key>=<value>` 查询链路过滤
    - 这样即使某个 `x-user-id` 有十几万唯一值，也不会把过滤器变成不可用的超大下拉
    - 关键改动：
      - `framework/logstore/store.go`
      - `framework/logstore/rdb.go`
      - `plugins/logging/utils.go`
      - `transports/bifrost-http/handlers/logging.go`
      - `ui/lib/store/apis/logsApi.ts`
      - `ui/app/workspace/logs/page.tsx`
      - `ui/components/filters/filterPopover.tsx`

  - **保留 `logging_headers` 的配置能力，但改成高性能消费方式**

    - `client.logging_headers` 中配置过的 header key 仍然会出现在 `metadata_keys`
    - 但只作为“可过滤字段名”暴露，不再预先聚合 value 集
    - 更适合后续把 `projectId / userId / tenantId` 这类业务标识放进 metadata 做日志分析

  - **Azure passthrough 回迁**

    - 基于 `prerelease3` 的安全增量，新增 Azure passthrough router
    - 这轮只扩展 integration router 注册，不改已有 provider queue、cluster fanout、governance/adaptive 主链
    - 关键改动：
      - `transports/bifrost-http/integrations/passthrough.go`
      - `transports/bifrost-http/handlers/integrations.go`

- **集群影响评估**

  - Metadata 过滤优化只改：
    - 日志过滤字段发现
    - 前端过滤器展示方式
    - 现有精确值查询入口
  - 没有改：
    - LLM 推理主链
    - provider queue
    - cluster config sync
    - adaptive / governance / vault / audit 的执行路径
  - 因此这轮不会破坏原有集群高可用链路；多节点下日志查询仍然走统一 LogStore，过滤语义保持一致。

- **本轮验证**

  - `go test ./logstore`
  - `go test ./logging`
  - `go test ./bifrost-http/handlers ./bifrost-http/server`
  - `go test ./bifrost-http -tags embedui -run TestDoesNotExist`
  - `npm exec tsc -- --noEmit`
  - `npm exec next build -- --no-lint`

- **新增回归覆盖**

  - metadata distinct 查询只返回 key，不再返回 value 列表
  - UI 过滤器与新的 `metadata_keys: string[]` 契约对齐
  - Azure passthrough integration 注册仍能通过 transport 入口编译验证

---

## 2026-04-15 09:18:26 CST | Base Commit 3eb9c7364 | prerelease3 MCP OAuth 引导信息前端落地

- **本轮目标**

  - 把前面已经回迁到后端的 `pending_oauth` 响应增强真正接到前端流程里，避免后端已经返回 `status_url / complete_url / next_steps`，但 UI 仍然只消费 `authorize_url` 的断层。
  - 在不改 cluster sync 主链的前提下，让 MCP OAuth 在企业多节点环境里的操作引导更完整。

- **本轮落地**

  - **MCP Registry 创建 OAuth MCP client 时，前端现在会完整消费 OAuth flow hints**

    - 创建响应中的以下字段会一起进入授权弹层：
      - `authorize_url`
      - `oauth_config_id`
      - `mcp_client_id`
      - `status_url`
      - `complete_url`
      - `next_steps`
    - OAuth 授权弹层新增：
      - 当前 `oauth_config_id / mcp_client_id` 上下文展示
      - `next_steps` 步骤说明
      - 重新打开授权窗口
      - 复制 `status_url`
      - 复制 `complete_url`
    - 关键改动：
      - `ui/app/workspace/mcp-registry/views/mcpClientForm.tsx`
      - `ui/app/workspace/mcp-registry/views/oauth2Authorizer.tsx`

  - **MCP Auth Config 管理页补充了更完整的操作入口**

    - 挂起中的 OAuth config 现在可以直接：
      - `Open Auth`
      - `Copy Status URL`
    - 待完成挂载的 authorized OAuth config 现在也可以：
      - `Copy Complete URL`
      - 继续执行 `Complete OAuth`
    - 原来只显示第一条 `next_steps`，现在改成完整步骤列表，便于运维排查和人工兜底操作
    - 关键改动：
      - `ui/app/workspace/mcp-auth-config/views/mcpAuthConfigView.tsx`

- **集群影响评估**

  - 这轮没有改：
    - OAuth config/token 的共享 ConfigStore
    - cluster fanout / oauth sync hook
    - MCP client create / complete / revoke 的后端协议
  - 只补了已有字段的前端消费与运维操作入口，因此不会改变多节点状态一致性的既有语义。
  - `status_url` / `complete_url` 仍然由当前访问节点生成；而 OAuth config 状态本身来自共享配置与已有 cluster sync，因此在集群下仍然是可用的一致状态。

- **本轮验证**

  - `go test ./bifrost-http/handlers -run 'TestAddMCPClientOAuthResponseIncludesGuidance|TestGetOAuthConfigsListsPendingAndLinkedClients'`
  - `npm exec tsc -- --noEmit`
  - `npm exec next build -- --no-lint`

---

## 2026-04-15 09:36:12 CST | Base Commit 3eb9c7364 | prerelease3 MCP OAuth 引导信息前端收口

- **本轮目标**

  - 在保持现有 OAuth config/token cluster sync 语义不变的前提下，把已经回迁到后端的 MCP OAuth guidance 字段真正用于前端操作流程。
  - 让 MCP Registry 和 MCP Auth Config 两个页面在企业多节点环境里都能更完整地引导授权和运维操作。

- **本轮落地**

  - **MCP Registry 的 OAuth 授权弹层增强**

    - 创建 OAuth MCP client 返回 `pending_oauth` 后，前端现在会完整消费这些字段：
      - `authorize_url`
      - `oauth_config_id`
      - `mcp_client_id`
      - `status_url`
      - `complete_url`
      - `next_steps`
    - 授权弹层新增：
      - `OAuth Context` 展示
      - `Open Authorization`
      - `Copy Status URL`
      - `Copy Complete URL`
      - 完整 `next_steps` 引导
    - 关键改动：
      - `ui/app/workspace/mcp-registry/views/mcpClientForm.tsx`
      - `ui/app/workspace/mcp-registry/views/oauth2Authorizer.tsx`

  - **MCP Auth Config 页面运维入口增强**

    - 挂起中的 OAuth config 现在可以直接：
      - 打开授权页
      - 复制 `status_url`
    - 已授权但尚未完成 MCP 挂载的 config 现在可以：
      - 复制 `complete_url`
      - 继续执行 `Complete OAuth`
    - `next_steps` 从“只显示第一条”改成完整步骤列表
    - 新增按钮都补了 `data-testid`，方便后续 E2E 或回归测试
    - 关键改动：
      - `ui/app/workspace/mcp-auth-config/views/mcpAuthConfigView.tsx`

- **集群影响评估**

  - 这轮没有改：
    - OAuth config/token 的共享 ConfigStore
    - OAuth sync hook
    - MCP create / complete / revoke 的后端协议
    - cluster fanout 主链
  - 因此不会改变多节点状态一致性的语义；它只是把现有一致状态更完整地呈现在前端。

- **本轮验证**

  - `go test ./bifrost-http/handlers -run 'TestAddMCPClientOAuthResponseIncludesGuidance|TestGetOAuthConfigsListsPendingAndLinkedClients'`
  - `npm exec next build -- --no-lint`
  - `npm exec tsc -- --noEmit`

### 2026-04-15 11:44:14 CST | Base Commit 3eb9c7364 | prerelease3 安全回迁（Pricing tier 增强）

> **说明**：这一轮优先回迁的是 `transports/v1.5.0-prerelease3` 里最适合安全吸收的 Pricing 增量，重点补齐 `272k token tier`、`priority tier`、`flex tier` 三组计费能力。改动全部收口在 `framework/modelcatalog` 和 `framework/configstore`，没有改推理主链、cluster fanout、adaptive/governance/vault/audit 的执行路径。

- **本轮落地**

  - **Model Catalog 新增 tier 定价字段**

    - 新增并接通了以下 pricing 字段：
      - `input_cost_per_token_flex`
      - `output_cost_per_token_flex`
      - `cache_read_input_token_cost_flex`
      - `input_cost_per_token_above_272k_tokens`
      - `input_cost_per_token_above_272k_tokens_priority`
      - `output_cost_per_token_above_272k_tokens`
      - `output_cost_per_token_above_272k_tokens_priority`
      - `cache_read_input_token_cost_above_272k_tokens`
      - `cache_read_input_token_cost_above_272k_tokens_priority`
      - `input_cost_per_token_above_200k_tokens_priority`
      - `output_cost_per_token_above_200k_tokens_priority`
      - `cache_read_input_token_cost_above_200k_tokens_priority`
    - 同步更新了：
      - pricing datasheet 反序列化结构
      - `TableModelPricing`
      - DB 表与模型之间的双向转换

  - **新增 `service_tier` 感知的定价分支**

    - `ChatResponse` 与 `ResponsesResponse` 现在会从响应中的 `service_tier` 推导出本次定价 tier：
      - `priority`
      - `flex`
      - 其它值按普通 tier 处理
    - 计费逻辑现在遵循：
      - `flex`：优先使用 flat flex rate，不再继续套 token tier
      - `priority`：优先使用对应的 priority tier rate
      - `272k`：优先级高于 `200k`
      - `cache_read`：支持 `flex / priority / 272k / 200k` 的层级回退
    - 关键改动：
      - `framework/modelcatalog/pricing.go`
      - `framework/modelcatalog/main.go`
      - `framework/modelcatalog/utils.go`

  - **数据库迁移**

    - `governance_model_pricing` 会新增以下列：
      - `input_cost_per_token_flex`
      - `output_cost_per_token_flex`
      - `cache_read_input_token_cost_flex`
      - `input_cost_per_token_above_272k_tokens`
      - `input_cost_per_token_above_272k_tokens_priority`
      - `output_cost_per_token_above_272k_tokens`
      - `output_cost_per_token_above_272k_tokens_priority`
      - `cache_read_input_token_cost_above_272k_tokens`
      - `cache_read_input_token_cost_above_272k_tokens_priority`
      - `input_cost_per_token_above_200k_tokens_priority`
      - `output_cost_per_token_above_200k_tokens_priority`
      - `cache_read_input_token_cost_above_200k_tokens_priority`
    - 新增 migration：
      - `add_priority_tier_pricing_columns`
      - `add_flex_tier_pricing_columns`

- **集群影响评估**

  - 这轮只涉及：
    - 共享 ConfigStore 的 pricing 表结构
    - 本地 `ModelCatalog` 的 cost 计算逻辑
  - 没有改：
    - cluster config propagation
    - provider queue
    - inference routing
    - adaptive/gossip/vault/audit 主链
  - 在集群部署下，只要各节点共用同一个 ConfigStore 并跑过 migration，就能得到一致的 pricing 行为。

- **环境变量 / 部署说明**

  - **没有新增环境变量**
  - 升级时需要注意：
    - 先让共享数据库执行 migration
    - 再滚动更新各个 Bifrost 节点
  - 如果当前部署显式停用了远端 pricing sync：
    - `pricing_url = ""`
    - 或 `pricing_sync_interval = 0`
    那这些新字段不会自动从远端 datasheet 拉取；只有本地/DB 已存在数据时才会生效。这是预期行为。

- **本轮验证**

  - `go test ./framework/modelcatalog`
  - `go test ./framework/configstore/...`
  - `go test ./plugins/logging ./plugins/governance`
  - `go test ./transports/bifrost-http/server ./transports/bifrost-http/handlers ./transports/bifrost-http/enterprise`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
  - `npm exec next build -- --no-lint`
  - `npm exec tsc -- --noEmit`

- **补充说明**

  - `npm exec tsc -- --noEmit` 仍然需要在 `next build` 之后执行，因为仓库当前 `tsconfig.json` 依赖 `.next/types`；这是仓库现状，不是这轮引入的问题。

### 2026-04-15 11:56:34 CST | Base Commit 3eb9c7364 | prerelease3 安全回迁（MCP 可观测增强 + 多节点定价 upsert 收口）

- **本轮目标**

  - 继续按 `transports/v1.5.0-prerelease3` 的低风险增量推进。
  - 优先补强：
    - MCP Registry 的离线/重连可观测性
    - 共享 ConfigStore 下 `ModelCatalog` 的多节点并发稳定性
  - 保持不改动：
    - 推理热路径
    - cluster config propagation 主链
    - adaptive / governance / vault / audit 的执行面

- **MCP Registry：把已持久化的工具快照真正用起来**

  - 后端 `GET /api/mcp/clients` 与分页接口现在会额外返回：
    - `tool_snapshot_source`
      - `live`
      - `persisted`
      - `none`
    - `tool_name_mapping`
  - 这样前端可以区分：
    - 当前看到的是运行时实时发现到的工具
    - 还是来自 ConfigStore 的最近一次成功快照
  - 关键改动：
    - `transports/bifrost-http/handlers/mcp.go`
    - `ui/lib/types/mcp.ts`
    - `ui/app/workspace/mcp-registry/views/mcpClientsTable.tsx`
    - `ui/app/workspace/mcp-registry/views/mcpClientSheet.tsx`

  - UI 行为增强：
    - MCP Registry 列表页的 `Enabled Tools` 现在会明确显示：
      - `Live tool discovery`
      - `Using ConfigStore snapshot`
    - MCP Client 详情页在使用持久化快照时，会显示 `ConfigStore snapshot` 标识和说明文案
    - 详情页新增 `Tool Discovery Snapshot` 区块，展示 `sanitized tool name -> original MCP tool name` 的映射 JSON，方便排查导入/同步/工具名裁剪问题

  - **集群语义**

    - 这次没有新增新的 MCP 同步协议。
    - 仍然复用已经存在的：
      - ConfigStore 持久化
      - MCP client cluster config sync
      - discovered tools snapshot persistence
    - 所以在多节点下，一个节点成功发现到的工具快照依然会通过共享配置面被其它节点看到；这次只是把这份状态更好地暴露给了前端。

- **ModelCatalog / Pricing：多节点并发 upsert 收口**

  - `governance_model_pricing` 的 upsert 现在改成单条原子 `ON CONFLICT` 语句，而不是“先查再写”。
  - `governance_model_parameters` 的 upsert 也做了同样的处理。
  - 这样在多节点同时启动、同时同步 model catalog / pricing 的场景下，更不容易出现锁竞争和死锁。
  - 关键改动：
    - `framework/configstore/rdb.go`
    - `framework/configstore/rdb_test.go`

  - **为什么这对集群更重要**

    - 原先的 find-then-save 流程在共享数据库下会放大并发竞争窗口。
    - 改成原子 upsert 后：
      - 多节点并发启动更稳
      - pricing/model parameters 初始化更幂等
      - 不会影响现有 pricing tier 逻辑

- **数据表 / 环境变量**

  - **本轮没有新增数据表**
  - **本轮没有新增环境变量**
  - `ModelCatalog` 这部分是 SQL 写法优化，不需要新配置
  - `MCP Registry` 这部分只是复用了已有的持久化字段和接口返回，不需要 migration

- **本轮验证**

  - `go test ./framework/configstore`
  - `go test ./framework/configstore/...`
  - `go test ./transports/bifrost-http/handlers -run 'TestGetMCPClientsPaginatedFallsBackToPersistedDiscoveredTools|TestGetMCPClientsPaginatedMarksLiveToolsWhenConnected|TestAddMCPClientPropagatesClusterConfigChange'`
  - `go test ./transports/bifrost-http/server -run TestDoesNotExist`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
  - `npm exec next build -- --no-lint`
  - `npm exec tsc -- --noEmit`

- **补充说明**

  - `npm exec next build -- --no-lint` 仍然会输出仓库现有的 `rewrites` / `output: export` warning，但构建已通过。
  - `npm exec tsc -- --noEmit` 仍然建议在 `next build` 之后执行，因为仓库当前依赖 `.next/types`；这是仓库现状，不是这轮引入的问题。

### 2026-04-15 15:12:08 CST | Base Commit 3eb9c7364 | prerelease3 安全回迁（MCP 导入校验收口 + 轻量 Observability 增量）

- **本轮目标**

  - 继续按 `transports/v1.5.0-prerelease3` 的安全增量推进。
  - 补齐一项轻量 observability 增量：
    - `content_summary` 真正暴露到日志接口与表格消息回退
  - 对 MCP 导入流做“诚实可用”的后端收口：
    - 支持导入前验证目标是否为 MCP-compatible endpoint
    - 对运行时模板头部场景明确标记为 `unverified`
    - 不再把“普通 REST API 已经自动转换成 MCP tool”这件事做成假象

- **Logs / Observability：`content_summary` 前后端打通**

  - 日志表中的 `content_summary` 现在会直接通过日志接口返回。
  - LLM Logs 列表页在没有结构化输入消息可展示时，会回退显示 `content_summary`，这对：
    - rerank
    - response 变体
    - disabled content logging 的元数据模式
    - 大 payload 场景
    更友好。
  - 关键改动：
    - `framework/logstore/tables.go`
    - `ui/lib/types/logs.ts`
    - `ui/app/workspace/logs/views/columns.tsx`

- **MCP 导入流：新增后端校验接口**

  - 新增：
    - `POST /api/mcp/client/validate`
  - 这个接口会在真正创建 MCP client 之前，对目标 endpoint 做分类：
    - `compatible`
      - 目标成功响应为 MCP-compatible HTTP/SSE server
      - 会返回探测到的 tool 名称列表
    - `unverified`
      - 当前配置依赖运行时模板头部，例如 `{{req.header.authorization}}`
      - 或需要 OAuth 交互授权，无法离线探测
      - 这类配置仍然允许导入，但会明确告知“未验证”
    - `incompatible`
      - 目标无法通过 MCP 初始化 / `tools/list`
      - 或根本不是 MCP-compatible endpoint

  - 关键改动：
    - `transports/bifrost-http/handlers/mcp.go`
    - `transports/bifrost-http/handlers/mcp_cluster_test.go`

- **MCP Registry 前端：导入流不再误导**

  - 导入弹窗标题与说明改为更准确的语义：
    - `Import MCP-Compatible Endpoints`
    - 明确说明当前 build 支持导入 `streamable HTTP / SSE MCP server`
    - 但还没有把普通企业 REST API 自动托管成 MCP tools
  - 导入页新增：
    - `Validate MCP Compatibility`
    - 每个 endpoint 的状态列
      - `MCP-compatible`
      - `Needs runtime auth`
      - `Not MCP-compatible`
  - 批量导入时会先调用后端验证：
    - `compatible` / `unverified` 才继续创建
    - `incompatible` 会被跳过并计入失败数
  - 手工新增 endpoint 时，也会先做验证再创建
  - 关键改动：
    - `ui/lib/types/mcp.ts`
    - `ui/lib/store/apis/mcpApi.ts`
    - `ui/app/workspace/mcp-registry/views/mcpImportDialog.tsx`
    - `ui/app/workspace/mcp-registry/page.tsx`

- **集群语义**

  - 本轮没有修改：
    - cluster config propagation 协议
    - MCP runtime 主执行链
    - tool sync / health monitor / OAuth sync hook
  - MCP 导入校验是“写入前检查”，不会影响现有多节点同步链。
  - `content_summary` 只是日志响应面增强，不会改变日志写入语义和多节点聚合逻辑。

- **数据表 / 环境变量**

  - **本轮没有新增数据表**
  - **本轮没有新增 migration**
  - **本轮没有新增环境变量**

- **本轮验证**

  - `go test ./framework/logstore -run 'TestLogCreateSerializesFields|TestGetDistinctMetadataKeysReturnsKeysOnly'`
  - `go test ./transports/bifrost-http/handlers -run 'TestValidateMCPClientReturnsUnverifiedForRuntimeTemplates|TestValidateMCPClientDetectsCompatibleEndpoint|TestGetMCPClientsPaginatedFallsBackToPersistedDiscoveredTools|TestGetMCPClientsPaginatedMarksLiveToolsWhenConnected|TestAddMCPClientPropagatesClusterConfigChange'`
  - `go test ./transports/bifrost-http/handlers -run TestDoesNotExist`
  - `go test ./transports/bifrost-http/server -run TestDoesNotExist`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
  - `npm exec next build -- --no-lint`

- **补充说明**

  - `npm exec tsc -- --noEmit` 在当前仓库仍会因为 `.next/types` 缺失报 `TS6053`；这是现有构建链问题，不是这轮改动引入的问题。
  - 这轮对 MCP 导入流做的是“真实能力收口”和“避免误导”，不是把官方 `MCP with Federated Auth` 里“普通私有 API 自动托管成 MCP tools”的完整 data plane 一次性做完。那一条后续仍可以继续单独推进。

### 2026-04-15 14:05:05 CST | Base Commit 3eb9c7364 | MCP Hosted Tools 落地（普通私有 API 托管为集群可同步的 MCP Tools）

- **MCP 导入流真正补上 Hosted Tools 执行面**

  - 新增了“Hosted MCP Tool”后端数据面：普通企业 HTTP API 不再只是被标记成 `incompatible/unverified`，而是可以直接落库并注册为本地 in-process MCP tool。
  - 当前支持的模板能力：
    - `{{req.header.<name>}}`
    - `{{env.<VAR>}}`
    - `{{args.<field>}}`
    - `{{req.body.<field>}}`
    - `{{req.query.<field>}}`
  - 导入后的 Hosted Tool 会在运行时把调用参数、请求头和环境变量解析后，再代理请求到目标企业 API。
  - 关键改动：
    - `core/mcp/interface.go`
    - `core/mcp/mcp.go`
    - `core/mcp/clientmanager.go`
    - `core/bifrost.go`
    - `transports/bifrost-http/server/mcp_hosted_tools.go`

- **新增 Hosted Tools 持久化与集群传播**

  - 新增 ConfigStore 表：
    - `config_mcp_hosted_tools`
  - 这个表用于存储 Hosted Tool 的：
    - `tool_id`
    - `name`
    - `method`
    - `url`
    - `headers`
    - `body_template`
    - `tool_schema`
  - 服务启动时会从 ConfigStore 重放注册 Hosted Tools。
  - 集群下新增了 `mcp_hosted_tool` 配置传播 scope，创建/删除 Hosted Tool 会像其它企业配置一样 fanout 到 peer 节点。
  - 删除链路已做成共享 ConfigStore 场景下的幂等语义，避免“源节点先删库，peer 再删时报 not found”导致 runtime 残留。
  - 关键改动：
    - `framework/configstore/tables/mcp_hosted_tool.go`
    - `framework/configstore/store.go`
    - `framework/configstore/rdb.go`
    - `framework/configstore/migrations.go`
    - `transports/bifrost-http/handlers/cluster_config_reload.go`
    - `transports/bifrost-http/server/cluster_config_propagation.go`

- **MCP HTTP Handler 与 UI 已完整适配**

  - 新增 API：
    - `GET /api/mcp/hosted-tools`
    - `POST /api/mcp/hosted-tool`
    - `DELETE /api/mcp/hosted-tool/{id}`
  - `Import MCP Endpoints` 页面现在会按分析结果自动分流：
    - `compatible`：继续创建真正的 MCP Server client
    - `unverified` / `incompatible`：直接创建 Hosted Tool
  - MCP Registry 页面新增 `Hosted API Tools` 区块，可查看并删除已托管工具。
  - 同时补了 tool name 规范化：
    - 导入名中的空格、连字符和非法字符会自动规范成 MCP 可执行的 tool name
    - 这样无论是 UI 导入、cluster fanout 还是服务启动重放，都能稳定注册
  - 关键改动：
    - `transports/bifrost-http/handlers/mcp.go`
    - `ui/lib/types/mcp.ts`
    - `ui/lib/store/apis/baseApi.ts`
    - `ui/lib/store/apis/mcpApi.ts`
    - `ui/app/workspace/mcp-registry/page.tsx`
    - `ui/app/workspace/mcp-registry/views/mcpImportDialog.tsx`
    - `ui/app/workspace/mcp-registry/views/mcpHostedToolsTable.tsx`

- **数据表 / 环境变量**

  - **本轮新增数据表**
    - `config_mcp_hosted_tools`
  - **本轮没有新增环境变量**
  - **部署注意**
    - 集群部署时，需要先对共享 ConfigStore 数据库执行 migration，再滚动更新各节点。

- **本轮验证**

  - `go test ./framework/configstore/...`
  - `go test ./transports/bifrost-http/handlers -run 'Test(AddMCPHostedToolPropagatesClusterConfigChange|DeleteMCPHostedToolPropagatesClusterConfigChange|GetMCPHostedToolsReturnsPersistedDefinitions|ValidateMCPClientReturnsUnverifiedForRuntimeTemplates|ValidateMCPClientDetectsCompatibleEndpoint|AddMCPClientOAuthResponseIncludesGuidance|GetMCPAuthConfigsListsPendingAndLinkedClients)$'`
  - `go test ./transports/bifrost-http/server -run 'Test(ApplyClusterConfigChangeMCPHostedToolLifecycle|ApplyClusterConfigChangeMCPClientLifecycle|PersistMCPDiscoveredToolsUpdatesStoreAndRuntimeConfig)$'`
  - `go test ./transports/bifrost-http/handlers ./transports/bifrost-http/server`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
  - `npm exec next build -- --no-lint`
  - `npm exec tsc -- --noEmit`

- **补充说明**

  - 这轮做的是 Hosted Tools 的第一版执行面与管理面，已经可以让普通私有 API 在现有企业版集群里被托管成可调用的 MCP tools。
  - 还没有继续往更深一层扩到 Postman/OpenAPI schema 的高级参数映射、响应结构化抽取、或更细粒度的 per-tool auth profile；这些可以在后续版本继续增强。

### 2026-04-15 14:31:15 CST | Base Commit 3eb9c7364 | Embedding 连接关闭兼容修复（fasthttp stale-connection retry 补强）

- **问题定位**

  - Embedding 请求报错：
    - `failed to execute HTTP request to provider API`
    - `the server closed connection before returning the first response byte. Make sure the server returns 'Connection: close' response header before closing the connection`
  - 这次定位结论是：
    - **不是 embedding 请求体格式问题**
    - **不是网关 `/v1/embeddings` handler 逻辑问题**
    - 更接近于 upstream provider server 在 keep-alive 连接上的关闭行为，与 `fasthttp` 连接复用的兼容边界
  - 直接用 curl 能成功，但通过网关偶发失败，是因为 curl 和 Bifrost 对连接复用/重试的行为不同。

- **修复内容**

  - 扩展了 stale connection retry 识别范围：
    - `fasthttp.ErrConnectionClosed`
    - `closed connection before returning the first response byte`
  - 这类错误现在会像 `io.EOF / broken pipe / connection reset by peer` 一样，在首次尝试时走一次安全重试。
  - 关键改动：
    - `core/network/http.go`
    - `core/network/http_test.go`
    - `core/providers/utils/dialer_test.go`

- **影响范围**

  - 只增强了 provider HTTP 请求的“连接关闭兼容重试”
  - 没有修改：
    - embedding 请求 schema
    - provider 路由逻辑
    - cluster fanout / HA 主链
    - governance / vault / adaptive / audit 执行面

- **数据表 / 环境变量**

  - **本轮没有新增数据表**
  - **本轮没有新增 migration**
  - **本轮没有新增环境变量**

- **本轮验证**

  - `go test ./core/network ./core/providers/utils`
  - `go test ./transports/bifrost-http/handlers ./transports/bifrost-http/server`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`

- **补充说明**

  - 这轮修复的是网关侧的兼容性缺口，所以对你们现网会更稳。
  - 但从根因上看，provider server 如果能在主动关闭 keep-alive 连接前正确返回 `Connection: close`，或者保证连接管理更规范，仍然是更理想的上游修复方式。

### 2026-04-15 15:00:54 CST | Base Commit 3eb9c7364 | Hosted MCP Tools 编辑能力增强（控制面低风险收口）

- **本轮目标**

  - 在不影响现有集群高可用主链、不触碰推理热路径的前提下，继续把 Hosted MCP Tools 的管理能力补齐。
  - 这轮重点补的是：
    - Hosted Tool 的后端更新能力
    - MCP Registry 的 Hosted Tool 编辑界面
    - 集群下 update / rename / delete 的 fanout 与幂等语义回归

- **实现内容**

  - Hosted MCP Tool 新增更新接口：
    - `PUT /api/mcp/hosted-tool/{id}`
  - 后端现在支持：
    - 读取已有 Hosted Tool
    - 更新 `name / method / url / headers / body_template / description`
    - 重新生成 `tool_schema`
    - 名称变化时先注销旧 runtime tool，再注册新 runtime tool
    - 更新 ConfigStore 后向集群广播 `mcp_hosted_tool` scope 的变更
  - 对应改动：
    - `transports/bifrost-http/handlers/mcp.go`
    - `transports/bifrost-http/server/mcp_hosted_tools.go`
    - `transports/bifrost-http/server/server.go`

- **前端增强**

  - MCP Registry 的 Hosted Tools 区块现在支持编辑：
    - 列表增加编辑按钮
    - 新增 Hosted Tool 表单弹层，可用于 create / edit 共用
    - 更新完成后会自动刷新 Hosted Tools 列表
  - 对应改动：
    - `ui/app/workspace/mcp-registry/views/mcpHostedToolsTable.tsx`
    - `ui/app/workspace/mcp-registry/views/mcpHostedToolForm.tsx`
    - `ui/lib/store/apis/mcpApi.ts`
    - `ui/lib/types/mcp.ts`

- **集群与性能说明**

  - 这轮改动全部位于 **控制面 CRUD / 配置同步**：
    - 不修改 inference hot path
    - 不修改 provider queue
    - 不修改 adaptive / governance / vault / audit 执行链
  - 集群下依赖的仍然是现有：
    - 共享 `ConfigStore`
    - `mcp_hosted_tool` cluster config fanout
    - peer apply 时的 add/update/delete 幂等逻辑
  - 因此这轮不会引入新的性能风险，影响范围仅限 Hosted Tool 管理操作本身。

- **数据表 / 环境变量**

  - **本轮没有新增数据表**
  - **本轮没有新增 migration**
  - **本轮没有新增环境变量**

- **本轮验证**

  - `go test ./framework/configstore/...`
  - `go test ./transports/bifrost-http/handlers -run 'Test(AddMCPHostedToolPropagatesClusterConfigChange|UpdateMCPHostedToolPropagatesClusterConfigChange|DeleteMCPHostedToolPropagatesClusterConfigChange|GetMCPHostedToolsReturnsPersistedDefinitions|ValidateMCPClientReturnsUnverifiedForRuntimeTemplates|ValidateMCPClientDetectsCompatibleEndpoint|AddMCPClientOAuthResponseIncludesGuidance|GetMCPAuthConfigsListsPendingAndLinkedClients)$'`
  - `go test ./transports/bifrost-http/server -run 'Test(ApplyClusterConfigChangeMCPHostedToolLifecycle|ApplyClusterConfigChangeMCPHostedToolRenameReplacesRuntimeRegistration|ApplyClusterConfigChangeMCPClientLifecycle|PersistMCPDiscoveredToolsUpdatesStoreAndRuntimeConfig)$'`
  - `go test ./transports/bifrost-http/handlers ./transports/bifrost-http/server`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
  - `npm exec next build -- --no-lint`

- **补充说明**

  - `npm exec tsc -- --noEmit` 依旧会受仓库现有 `.next/types` 产物缺失影响报 `TS6053`，这不是本轮改动引入的问题。
  - 这轮完成后，Hosted MCP Tools 已经具备更完整的 create / update / delete 管理能力，适合继续往更深一层做参数映射与响应结构化增强。

### 2026-04-15 15:18:40 CST | Base Commit 3eb9c7364 | Hosted MCP Tools 参数映射与结构化响应增强

- **本轮目标**

  - 在保持集群高可用、高性能主链不受影响的前提下，把 Hosted MCP Tools 从“基础 HTTP 代理”增强到更适合企业 API 托管的形态。
  - 这轮重点补了两类能力：
    - 显式 `query_params` 参数映射
    - `response_json_path / response_template` 结构化响应处理

- **实现内容**

  - Hosted Tool 配置新增：
    - `query_params`
    - `response_json_path`
    - `response_template`
  - 请求执行层增强：
    - URL 仍支持原有模板替换
    - 额外支持把 `query_params` 作为独立映射拼到最终请求 URL
    - `body_template` 继续保留
    - `response_json_path` 可从 JSON 返回中直接抽取目标字段
    - `response_template` 支持 `{{response.*}}` 模板渲染，并优先于 `response_json_path`
    - 保留 `{{req.header.*}}`、`{{env.*}}`、`{{args.*}}`、`{{req.body.*}}`、`{{req.query.*}}`
  - 关键改动：
    - `framework/configstore/tables/mcp_hosted_tool.go`
    - `framework/configstore/rdb.go`
    - `framework/configstore/migrations.go`
    - `transports/bifrost-http/handlers/mcp.go`
    - `transports/bifrost-http/server/mcp_hosted_tools.go`

- **前端增强**

  - Hosted Tool 表单现在支持：
    - 显式编辑 Query Params
    - 配置 Response JSON Path
    - 配置 Response Template
  - Hosted Tools 列表增加了：
    - Query Params 数量展示
    - Response 模式展示（Raw / JSON Path / Template）
  - 对应改动：
    - `ui/app/workspace/mcp-registry/views/mcpHostedToolForm.tsx`
    - `ui/app/workspace/mcp-registry/views/mcpHostedToolsTable.tsx`
    - `ui/lib/types/mcp.ts`

- **集群与性能说明**

  - 这轮增强仍然只发生在 Hosted Tool 的控制面和工具执行层：
    - 不修改 inference hot path
    - 不修改 provider queue
    - 不修改 cluster membership / health / config fanout 主链
  - 集群一致性仍依赖：
    - 共享 `ConfigStore`
    - 现有 `mcp_hosted_tool` cluster config fanout
    - peer apply 时的 add/update/delete 幂等逻辑
  - 性能方面：
    - `query_params` 只在 Hosted Tool 执行时做一次 URL 拼接
    - `response_json_path / response_template` 只在 Hosted Tool 收到响应后做本地处理
    - 不会把额外开销带到普通 LLM inference 请求路径

- **数据表 / 环境变量**

  - **本轮新增 migration**
    - `add_mcp_hosted_tool_enhancement_columns`
  - **本轮新增列**
    - `config_mcp_hosted_tools.query_params_json`
    - `config_mcp_hosted_tools.response_json_path`
    - `config_mcp_hosted_tools.response_template`
  - **本轮没有新增环境变量**
  - 集群部署注意事项：
    - 先对共享数据库执行 migration
    - 再对各节点做滚动更新

- **本轮验证**

  - `go test ./framework/configstore/...`
  - `go test ./transports/bifrost-http/server -run 'Test(ExecuteHostedMCPToolAppliesQueryParamsAndExtractsJSONPath|ExecuteHostedMCPToolAppliesResponseTemplate|ExecuteHostedMCPToolSupportsResponseRawTemplate|ExecuteHostedMCPToolUsesRequestHeadersInQueryAndBodyTemplates|ApplyClusterConfigChangeMCPHostedToolLifecycle|ApplyClusterConfigChangeMCPHostedToolRenameReplacesRuntimeRegistration|ApplyClusterConfigChangeMCPClientLifecycle|PersistMCPDiscoveredToolsUpdatesStoreAndRuntimeConfig)$'`
  - `go test ./transports/bifrost-http/handlers -run 'Test(AddMCPHostedToolPropagatesClusterConfigChange|UpdateMCPHostedToolPropagatesClusterConfigChange|DeleteMCPHostedToolPropagatesClusterConfigChange|GetMCPHostedToolsReturnsPersistedDefinitions|ValidateMCPClientReturnsUnverifiedForRuntimeTemplates|ValidateMCPClientDetectsCompatibleEndpoint|AddMCPClientOAuthResponseIncludesGuidance|GetMCPAuthConfigsListsPendingAndLinkedClients)$'`
  - `go test ./transports/bifrost-http/handlers ./transports/bifrost-http/server`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
  - `npm exec next build -- --no-lint`
  - `npm exec tsc -- --noEmit`

- **补充说明**

  - 这轮完成后，Hosted MCP Tools 已经不只是“代发一个固定 HTTP 请求”，而是具备了更实用的企业 API 参数映射与响应抽取能力。
  - 下一步如果继续增强，最值得做的是：
    - 更细粒度的 per-tool auth/profile
    - 更丰富的 response schema 提示
    - 更强的 OpenAPI/Postman 导入映射自动化

---

### 2026-04-15 15:43:41 CST | Base Commit 3eb9c7364 | Hosted MCP Tools：per-tool auth/profile、自动映射增强与内存风险收口

- **实现内容**

  - Hosted Tool 新增了更完整的 per-tool profile：
    - `auth_profile`
      - `none`
      - `bearer_passthrough`
      - `header_passthrough`
    - `execution_profile`
      - `timeout_seconds`
      - `max_response_body_bytes`
  - 执行层增强：
    - Hosted Tool 在调用上游企业 API 时，支持按 tool 级别复用调用方 `Authorization` 或指定请求头
    - 支持按 tool 级别设置超时和响应体大小上限
    - `response_template` 只有在确实用到 `{{response.raw}}` 时才会额外构造原始响应字符串，避免无谓的大对象复制
  - OpenAPI / Postman / cURL / Manual 导入增强：
    - 自动拆分 URL query string 到 `query_params`
    - OpenAPI 支持把 `query` / `path` 参数映射成 Hosted Tool 参数模板
    - 安全头部如 `Authorization: {{req.header.authorization}}`、`X-Tenant-ID: {{req.header.x-tenant-id}}` 会自动归并成 `auth_profile`
    - Manual 导入在落 Hosted Tool 时也会自动做 URL query 与 auth profile 收口

- **关键改动**

  - 后端：
    - `framework/configstore/tables/mcp_hosted_tool.go`
    - `framework/configstore/rdb.go`
    - `framework/configstore/migrations.go`
    - `transports/bifrost-http/handlers/mcp.go`
    - `transports/bifrost-http/server/mcp_hosted_tools.go`
    - `transports/bifrost-http/server/mcp_hosted_tools_test.go`
  - 前端：
    - `ui/app/workspace/mcp-registry/views/mcpHostedToolForm.tsx`
    - `ui/app/workspace/mcp-registry/views/mcpHostedToolsTable.tsx`
    - `ui/app/workspace/mcp-registry/views/mcpImportDialog.tsx`
    - `ui/lib/types/mcp.ts`

- **集群、高可用与性能说明**

  - 这轮增强仍然沿用原有模式：
    - Hosted Tool 配置持久化到共享 `ConfigStore`
    - 通过既有 `mcp_hosted_tool` cluster config fanout 传播
    - peer 侧按现有 add / update / delete 幂等 apply 逻辑生效
  - **没有引入新的集群状态分叉机制**
  - **没有修改普通 LLM inference hot path**
  - 内存风险收口重点：
    - Hosted Tool 原本会完整缓冲上游响应体，这在上游返回大 payload 时会直接放大内存占用
    - 本轮通过 `execution_profile.max_response_body_bytes` 增加了 tool 级硬限制
    - 仅在 `response_template` 真正引用 `response.raw` 时才会额外复制原始响应内容
  - 代码审视结果：
    - 本轮新增逻辑没有引入新的常驻 goroutine
    - 没有引入新的全局缓存或无限增长的 map
    - 当前更容易放大内存占用的场景主要仍是：
      - Hosted Tool 上游返回超大响应
      - 开启正文日志时的大报文日志写入
      - Streaming 请求的全量响应累积
    - 这次已经把 Hosted Tool 这条链的主要风险做了可配置硬限制

- **数据表 / Migration / 环境变量**

  - **本轮新增列**
    - `config_mcp_hosted_tools.auth_profile_json`
    - `config_mcp_hosted_tools.execution_profile_json`
  - **本轮沿用并扩展现有 migration**
    - `add_mcp_hosted_tool_enhancement_columns`
  - **本轮没有新增环境变量**
  - 集群部署注意事项：
    - 先在共享数据库上完成 migration
    - 再对各节点滚动发布

- **本轮验证**

  - `go test ./framework/configstore/...`
  - `go test ./transports/bifrost-http/server -run 'Test(ExecuteHostedMCPToolAppliesQueryParamsAndExtractsJSONPath|ExecuteHostedMCPToolAppliesResponseTemplate|ExecuteHostedMCPToolSupportsResponseRawTemplate|ExecuteHostedMCPToolUsesRequestHeadersInQueryAndBodyTemplates|ExecuteHostedMCPToolAppliesBearerPassthroughAuthProfile|ExecuteHostedMCPToolAppliesHeaderPassthroughAuthProfile|ExecuteHostedMCPToolRespectsExecutionProfileMaxResponseBodyBytes|ExecuteHostedMCPToolRespectsExecutionProfileTimeout|ApplyClusterConfigChangeMCPHostedToolLifecycle|ApplyClusterConfigChangeMCPHostedToolRenameReplacesRuntimeRegistration|ApplyClusterConfigChangeMCPClientLifecycle|PersistMCPDiscoveredToolsUpdatesStoreAndRuntimeConfig)$'`
  - `go test ./transports/bifrost-http/handlers -run 'Test(AddMCPHostedToolPropagatesClusterConfigChange|UpdateMCPHostedToolPropagatesClusterConfigChange|DeleteMCPHostedToolPropagatesClusterConfigChange|GetMCPHostedToolsReturnsPersistedDefinitions|ValidateMCPClientReturnsUnverifiedForRuntimeTemplates|ValidateMCPClientDetectsCompatibleEndpoint|AddMCPClientOAuthResponseIncludesGuidance|GetMCPAuthConfigsListsPendingAndLinkedClients)$'`
  - `go test ./transports/bifrost-http/handlers ./transports/bifrost-http/server`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist`
  - `npm exec next build -- --no-lint`
  - `npm exec tsc -- --noEmit`

- **补充说明**

  - 这轮完成后，Hosted MCP Tools 已经具备：
    - 参数映射
    - 响应结构化
    - per-tool auth/profile
    - 更强的 OpenAPI / Postman / cURL / Manual 自动映射
  - 下一步如果继续增强，最值得做的是：
    - 更强的 OpenAPI schema → tool parameter 类型映射
    - Hosted Tool 的响应 schema/preview
    - 更细粒度的 per-tool observability 与调用统计

---

### 2026-04-15 16:05:31 CST | Base Commit 3eb9c7364 | Hosted Tool：OpenAPI 参数类型映射与导入链收口

- **实现内容**

  - Hosted Tool 创建/更新接口现在支持显式传入 `tool_schema`
  - 当导入源已经能提供更完整的参数定义时，不再只靠模板字符串推断参数名，而是保留：
    - 参数类型
    - `required`
    - 参数描述
    - 数组/对象的基础 schema 结构
  - OpenAPI 导入增强：
    - `query` / `path` 参数会映射成更完整的 tool parameter schema
    - `requestBody.application/json` 的 object schema 会映射成：
      - Hosted Tool 参数 schema
      - 对应的 `body_template`
    - OpenAPI `securitySchemes` 和运行时 header 模板仍会继续自动收口成 `auth_profile`
  - Postman / cURL / Manual 导入增强：
    - 继续自动拆 URL query
    - 继续自动识别 `Authorization` / 自定义 header passthrough
    - Hosted Tool 创建 payload 里会统一带上归一化后的 `tool_schema`

- **关键改动**

  - 后端：
    - `transports/bifrost-http/handlers/mcp.go`
  - 前端：
    - `ui/app/workspace/mcp-registry/views/mcpImportDialog.tsx`
    - `ui/lib/types/mcp.ts`

- **集群、高可用与性能说明**

  - 这轮只增强 Hosted Tool 的控制面和导入映射，不修改 inference hot path
  - `tool_schema` 仍通过原有 Hosted Tool 配置存储在共享 `ConfigStore` 中，并沿用现有 cluster fanout 同步
  - 没有引入新的运行时共享状态，也没有新增 cluster sidecar/worker
  - 性能影响主要发生在导入阶段和 Hosted Tool 注册阶段，不在普通推理请求路径上

- **数据表 / Migration / 环境变量**

  - **本轮没有新增数据表**
  - **本轮没有新增 migration**
  - **本轮没有新增环境变量**

- **本轮验证**

  - `go test ./transports/bifrost-http/handlers -run 'Test(AddMCPHostedToolPropagatesClusterConfigChange|UpdateMCPHostedToolPropagatesClusterConfigChange|DeleteMCPHostedToolPropagatesClusterConfigChange|AddMCPHostedToolPreservesProvidedToolSchema|GetMCPHostedToolsReturnsPersistedDefinitions|ValidateMCPClientReturnsUnverifiedForRuntimeTemplates|ValidateMCPClientDetectsCompatibleEndpoint|AddMCPClientOAuthResponseIncludesGuidance|GetMCPAuthConfigsListsPendingAndLinkedClients)$'`
  - `go test ./transports/bifrost-http/server -run 'Test(ExecuteHostedMCPToolAppliesQueryParamsAndExtractsJSONPath|ExecuteHostedMCPToolAppliesResponseTemplate|ExecuteHostedMCPToolSupportsResponseRawTemplate|ExecuteHostedMCPToolUsesRequestHeadersInQueryAndBodyTemplates|ExecuteHostedMCPToolAppliesBearerPassthroughAuthProfile|ExecuteHostedMCPToolAppliesHeaderPassthroughAuthProfile|ExecuteHostedMCPToolRespectsExecutionProfileMaxResponseBodyBytes|ExecuteHostedMCPToolRespectsExecutionProfileTimeout|ApplyClusterConfigChangeMCPHostedToolLifecycle|ApplyClusterConfigChangeMCPHostedToolRenameReplacesRuntimeRegistration|ApplyClusterConfigChangeMCPClientLifecycle|PersistMCPDiscoveredToolsUpdatesStoreAndRuntimeConfig)$'`
  - `npm exec next build -- --no-lint`
  - `npm exec tsc -- --noEmit`

- **补充说明**

  - 到这一轮为止，Hosted Tool 已经不是“只把 URL + headers 包起来”的轻薄壳了，而是具备：
    - 参数映射
    - 参数类型/schema
    - per-tool auth/profile
    - 响应结构化
    - 导入自动收口
  - 下一步如果继续增强，最值得做的是：
    - Hosted Tool 的响应 schema / preview
    - 更细粒度的 Hosted Tool 调用观测
    - OpenAPI / Postman 更完整的 schema 与 example 映射

### 2026-04-15 19:16:00 CST | Base Commit 3eb9c7364 | Hosted Tool：响应 Schema、Preview 与轻量执行观测

- **本轮目标**

  - 补齐 Hosted Tool 的响应 schema/preview 能力
  - 给 Hosted Tool 增加更细粒度但仍然轻量的执行观测
  - 保持集群高可用语义不变，不把新逻辑带入普通 LLM inference hot path

- **本轮完成**

  - Hosted Tool 新增 `response_schema` 持久化字段，支持从 OpenAPI 导入和在 UI 中手工维护
  - Hosted Tool 新增预览接口：`POST /api/mcp/hosted-tool/{id}/preview`
  - Preview 返回结构化执行元数据：
    - `status_code`
    - `latency_ms`
    - `response_bytes`
    - `content_type`
    - `resolved_url`
    - `truncated`
    - `response_schema`
  - Preview 执行会继承当前请求头上下文，因此 `bearer_passthrough` / `header_passthrough` 类型的 Hosted Tool 预览也能复用现有请求头
  - Preview 输出增加了 64KB 上限保护，避免管理面一次预览把超大响应直接灌进浏览器或在网关侧制造不必要的大对象
  - MCP Registry 的 Hosted Tools 列表新增：
    - `Schema` 状态展示
    - `Preview` 操作入口
  - Hosted Tool 编辑表单新增 `Response Schema (optional)` JSON 编辑区
  - OpenAPI 导入链新增 response schema 映射，会把 2xx JSON response schema 自动带进 Hosted Tool 配置

- **关键改动**

  - 后端：
    - `framework/configstore/tables/mcp_hosted_tool.go`
    - `framework/configstore/rdb.go`
    - `framework/configstore/migrations.go`
    - `transports/bifrost-http/server/mcp_hosted_tools.go`
    - `transports/bifrost-http/handlers/mcp.go`
  - 前端：
    - `ui/lib/types/mcp.ts`
    - `ui/lib/store/apis/mcpApi.ts`
    - `ui/app/workspace/mcp-registry/views/mcpHostedToolForm.tsx`
    - `ui/app/workspace/mcp-registry/views/mcpHostedToolsTable.tsx`
    - `ui/app/workspace/mcp-registry/views/mcpHostedToolPreviewDialog.tsx`
    - `ui/app/workspace/mcp-registry/views/mcpImportDialog.tsx`

- **集群、高可用与性能说明**

  - 这轮新增的是 Hosted Tool 的配置字段和显式 preview 控制面，不改普通模型推理主链
  - `response_schema` 和其它 Hosted Tool 配置一样，继续走共享 `ConfigStore` + 现有 cluster fanout，同步语义不变
  - Preview 是显式按需调用，不引入新的后台 worker、poller 或 gossip 通道
  - Preview 仍复用已有 per-tool：
    - `timeout_seconds`
    - `max_response_body_bytes`
  - 新增的 64KB preview 输出截断可以减少管理面的大响应内存占用风险
  - 本轮没有新增常驻 goroutine、无界缓存或新的热路径共享状态，因此不会放大集群高可用主链的运行风险

- **数据表 / Migration / 环境变量**

  - **本轮没有新增数据表**
  - **本轮新增 migration 列**
    - `config_mcp_hosted_tools.response_schema_json`
  - **本轮没有新增环境变量**

- **部署注意**

  - 集群升级顺序：
    1. 先升级共享数据库 migration
    2. 再滚动更新各节点
  - Preview 接口不要求额外 sidecar 或独立服务

- **本轮验证**

  - `go test ./configstore/...` in `framework`
  - `go test ./bifrost-http/handlers -run 'Test(AddMCPHostedToolPropagatesClusterConfigChange|UpdateMCPHostedToolPropagatesClusterConfigChange|DeleteMCPHostedToolPropagatesClusterConfigChange|AddMCPHostedToolPreservesProvidedToolSchema|PreviewMCPHostedToolReturnsExecutionMetadata|GetMCPHostedToolsReturnsPersistedDefinitions|ValidateMCPClientReturnsUnverifiedForRuntimeTemplates|ValidateMCPClientDetectsCompatibleEndpoint|AddMCPClientOAuthResponseIncludesGuidance|GetMCPAuthConfigsListsPendingAndLinkedClients)$'` in `transports`
  - `go test ./bifrost-http/server -run 'Test(ExecuteHostedMCPToolAppliesQueryParamsAndExtractsJSONPath|ExecuteHostedMCPToolAppliesResponseTemplate|ExecuteHostedMCPToolSupportsResponseRawTemplate|ExecuteHostedMCPToolUsesRequestHeadersInQueryAndBodyTemplates|ExecuteHostedMCPToolAppliesBearerPassthroughAuthProfile|ExecuteHostedMCPToolAppliesHeaderPassthroughAuthProfile|ExecuteHostedMCPToolRespectsExecutionProfileMaxResponseBodyBytes|ExecuteHostedMCPToolRespectsExecutionProfileTimeout|ExecuteHostedMCPToolWithMetadataCapturesExecutionDetails|PreviewMCPHostedToolTruncatesLargeOutput|ApplyClusterConfigChangeMCPHostedToolLifecycle|ApplyClusterConfigChangeMCPHostedToolRenameReplacesRuntimeRegistration|ApplyClusterConfigChangeMCPClientLifecycle|PersistMCPDiscoveredToolsUpdatesStoreAndRuntimeConfig)$'` in `transports`
  - `go test ./bifrost-http -tags embedui -run TestDoesNotExist` in `transports`
  - `npm exec next build -- --no-lint` in `ui`
  - `npm exec next typegen` in `ui`
  - `npm exec tsc -- --noEmit` in `ui`

- **补充说明**

  - 这轮的“更细粒度调用观测”是轻量版本，重点在单次 preview 的执行元数据，而不是引入新的高频持久化 metrics 面
  - 如果继续增强，下一步最值得做的是：
    - Hosted Tool 的 response examples / sample payload 映射
    - Hosted Tool 的调用日志聚合视图
    - OpenAPI response schema 到 UI 参数/结果展示的进一步自动化

### 2026-04-15 20:24:00 CST | Base Commit 3eb9c7364 | Hosted Tool：Response Examples 与轻量调用观测

- **目标**

  - 在不引入新后台 worker、不改普通 LLM inference hot path 的前提下，把 Hosted Tool 再往“可运营、可排障”推进一层
  - 继续保持集群部署下的共享 `ConfigStore` + cluster fanout 语义，不制造新的状态分叉
  - 继续把 OpenAPI / 导入链和 Hosted Tool 表单收口成一套一致的能力面

- **本轮完成**

  - Hosted Tool 新增 `response_examples` 持久化字段，支持：
    - UI 手工维护
    - OpenAPI 2xx `example/examples` 自动导入
  - MCP Registry 的 Hosted Tools 列表新增：
    - `Examples` 状态展示
    - `Observability` 操作入口
  - 新增 Hosted Tool 轻量观测弹层：
    - 按 `24h / 7d / 30d` 查看最近调用
    - 展示 `total calls / success rate / avg latency / total cost`
    - 展示最近 10 条调用日志
    - 复用现有 MCP logs/logstore，不新增专用 metrics 存储
  - Observability 弹层会直接展示：
    - 配置中的 `response_examples`
    - 配置中的 `response_schema`
    - 最近调用状态、延迟、Virtual Key、LLM Request、错误摘要

- **关键改动**

  - 后端：
    - `framework/configstore/tables/mcp_hosted_tool.go`
    - `framework/configstore/rdb.go`
    - `framework/configstore/migrations.go`
    - `transports/bifrost-http/handlers/mcp.go`
  - 前端：
    - `ui/lib/types/mcp.ts`
    - `ui/app/workspace/mcp-registry/views/mcpHostedToolForm.tsx`
    - `ui/app/workspace/mcp-registry/views/mcpHostedToolsTable.tsx`
    - `ui/app/workspace/mcp-registry/views/mcpHostedToolObservabilityDialog.tsx`
    - `ui/app/workspace/mcp-registry/views/mcpImportDialog.tsx`

- **集群、高可用与性能说明**

  - 本轮没有改普通模型推理主链，也没有新增后台同步线程
  - Hosted Tool 的新字段继续走共享 `ConfigStore` + 现有 cluster fanout，同步语义不变
  - 轻量观测弹层直接查询现有 MCP logs，只有在用户打开弹层时才触发请求，不做轮询
  - 这样可以复用已有日志索引和过滤能力，避免再引入一套新的高频聚合存储
  - `response_examples` 仅作为控制面配置，不参与运行时请求执行，不会放大主链延迟

- **数据表 / Migration / 环境变量**

  - **本轮没有新增数据表**
  - **本轮新增 migration 列**
    - `config_mcp_hosted_tools.response_examples_json`
  - **本轮没有新增环境变量**

- **部署注意**

  - 集群升级顺序：
    1. 先升级共享数据库 migration
    2. 再滚动更新各节点
  - 观测弹层依赖现有 MCP logs；如果某环境关闭了日志存储，只会没有历史调用数据，不影响 Hosted Tool 本身执行

- **本轮验证**

  - `go test ./configstore/...` in `framework`
  - `go test ./bifrost-http/handlers -run 'Test(GetMCPHostedToolsReturnsPersistedDefinitions|AddMCPHostedToolPropagatesClusterConfigChange|UpdateMCPHostedToolPropagatesClusterConfigChange|DeleteMCPHostedToolPropagatesClusterConfigChange|AddMCPHostedToolPreservesProvidedToolSchema|PreviewMCPHostedToolReturnsExecutionMetadata)$'` in `transports`
  - `go test ./bifrost-http/server -run 'Test(ExecuteHostedMCPToolAppliesQueryParamsAndExtractsJSONPath|ExecuteHostedMCPToolAppliesResponseTemplate|ExecuteHostedMCPToolSupportsResponseRawTemplate|ExecuteHostedMCPToolUsesRequestHeadersInQueryAndBodyTemplates|ExecuteHostedMCPToolAppliesBearerPassthroughAuthProfile|ExecuteHostedMCPToolAppliesHeaderPassthroughAuthProfile|ExecuteHostedMCPToolRespectsExecutionProfileMaxResponseBodyBytes|ExecuteHostedMCPToolRespectsExecutionProfileTimeout|ExecuteHostedMCPToolWithMetadataCapturesExecutionDetails|PreviewMCPHostedToolTruncatesLargeOutput|ApplyClusterConfigChangeMCPHostedToolLifecycle|ApplyClusterConfigChangeMCPHostedToolRenameReplacesRuntimeRegistration|ApplyClusterConfigChangeMCPClientLifecycle|PersistMCPDiscoveredToolsUpdatesStoreAndRuntimeConfig)$'` in `transports`
  - `go test ./bifrost-http/handlers ./bifrost-http/server` in `transports`
  - `go test ./bifrost-http -tags embedui -run TestDoesNotExist` in `transports`
  - `npm exec next build -- --no-lint` in `ui`
  - `npm exec next typegen` in `ui`
  - `npm exec tsc -- --noEmit` in `ui`

### 2026-04-15 21:08:00 CST | Base Commit 3eb9c7364 | Hosted Tool：参数校验与执行前 Schema 校验

- **目标**

  - 在不影响普通 LLM 推理 hot path 的前提下，给 Hosted Tool 补上真正的输入参数校验
  - 让 OpenAPI / 手工定义的 `tool_schema` 不只是展示用途，而是在执行前真正生效
  - 继续保持集群部署下的共享 `ConfigStore` + cluster fanout 语义不变

- **本轮完成**

  - Hosted Tool 在执行前会按 `tool_schema.function.parameters` 做参数校验
  - 当前支持的校验能力包括：
    - `required`
    - `type`
    - `enum`
    - `minimum / maximum`
    - `minLength / maxLength`
    - `pattern`
    - `minItems / maxItems`
    - `additionalProperties`
    - `anyOf / oneOf / allOf`
    - 本地 `$ref`（`#/$defs/*`、`#/definitions/*`）
  - 如果参数不满足 schema，会在真正发起上游 HTTP 请求前直接失败，避免无效请求打到后端系统
  - 这层校验仅作用于 Hosted Tool 执行链，不进入普通 provider inference 主链

- **关键改动**

  - 后端：
    - `transports/bifrost-http/server/mcp_hosted_tools.go`
    - `transports/bifrost-http/server/mcp_hosted_tools_test.go`

- **集群、高可用与性能说明**

  - 这轮没有新增后台 worker、poller、gossip 或额外 cluster 状态同步面
  - 校验逻辑只在 Hosted Tool 被调用时运行一次，不影响普通模型请求路径
  - 没有改共享 `ConfigStore` 的结构和 cluster fanout 语义，所以多节点下的配置一致性逻辑保持不变
  - 参数校验发生在本地执行前，可以减少无效请求打到内网 API，从稳定性角度是收敛而不是放大风险

- **数据表 / Migration / 环境变量**

  - **本轮没有新增数据表**
  - **本轮没有新增 migration**
  - **本轮没有新增环境变量**

- **部署注意**

  - 这轮只涉及 Hosted Tool 执行层代码和测试
  - 不需要额外变更数据库结构，按现有滚动更新方式部署即可

- **本轮验证**

  - `go test ./transports/bifrost-http/server -run 'Test(ExecuteHostedMCPToolAppliesQueryParamsAndExtractsJSONPath|ExecuteHostedMCPToolAppliesResponseTemplate|ExecuteHostedMCPToolSupportsResponseRawTemplate|ExecuteHostedMCPToolUsesRequestHeadersInQueryAndBodyTemplates|ExecuteHostedMCPToolRejectsMissingRequiredArgs|ExecuteHostedMCPToolRejectsWrongArgType|ExecuteHostedMCPToolRejectsUnknownArgsWhenAdditionalPropertiesDisabled|ExecuteHostedMCPToolAcceptsValidArgsAgainstSchema|ExecuteHostedMCPToolAppliesBearerPassthroughAuthProfile|ExecuteHostedMCPToolAppliesHeaderPassthroughAuthProfile|ExecuteHostedMCPToolRespectsExecutionProfileMaxResponseBodyBytes|ExecuteHostedMCPToolRespectsExecutionProfileTimeout|ExecuteHostedMCPToolWithMetadataCapturesExecutionDetails|PreviewMCPHostedToolTruncatesLargeOutput|ApplyClusterConfigChangeMCPHostedToolLifecycle|ApplyClusterConfigChangeMCPHostedToolRenameReplacesRuntimeRegistration|ApplyClusterConfigChangeMCPClientLifecycle|PersistMCPDiscoveredToolsUpdatesStoreAndRuntimeConfig)$'` in `transports`
  - `go test ./transports/bifrost-http/handlers -run TestDoesNotExist` in `transports`
  - `go test ./transports/bifrost-http -tags embedui -run TestDoesNotExist` in `transports`

### 2026-04-15 22:06:00 CST | Base Commit 3eb9c7364 | Provider Network：超时语义说明增强与 LiteLLM 对照

- **目标**

  - 把 Provider Network 页里的 timeout 语义讲清楚，避免继续把 `timeout` 误解成“首 token 超时”
  - 对照 LiteLLM 的 timeout 设计，确认当前 Bifrost 的行为边界与可参考方向

- **本轮完成**

  - Provider Network 页面把 `Timeout (seconds)` 明确改成了 `Request Timeout (seconds)`
  - 新增了更直白的说明：
    - `Request Timeout` 是上游 provider HTTP 请求的整体时间预算，不是首 token 专用超时
    - `Stream Idle Timeout` 只约束 streaming 响应中 chunk 与 chunk 之间的静默间隔，不限制整条流总时长
    - 请求如果先在队列里等待，再发往 provider，通常是“排队时间”和“provider 请求超时”分开计算
    - 给出了长推理/长流式输出的建议起始值：`Request Timeout = 300-600s`、`Stream Idle Timeout = 60-120s`

- **LiteLLM 对照结论**

  - LiteLLM 官方文档把 `timeout` 明确解释为**整个调用的总时长上限**，并在 Router 级向下传递到 completion 调用层
  - LiteLLM 还额外区分了 `stream_timeout`，官方文档把它定义成**流式请求等待首个 chunk / 首 token 的时间上限**
  - LiteLLM Router 的超时优先级更细：
    - 每次请求传入的 `timeout / request_timeout`
    - deployment `litellm_params.timeout / request_timeout`
    - Router 全局 `timeout`
    - streaming 时还会优先取 `stream_timeout`
  - LiteLLM 还支持更细粒度的 HTTP timeout/config 方式：
    - 直接传 `httpx.Timeout`
    - 注入自定义 `aiohttp.ClientSession`
    - 自定义连接池、keepalive、limit_per_host、connect/read/total timeout 等
  - 当前 Bifrost **没有** LiteLLM 这套“总超时 + 首 chunk 专用超时 + 细粒度 HTTP client timeout 对象”三层模型；当前是：
    - `default_request_timeout_in_seconds`：更广义的 provider 请求级网络超时
    - `stream_idle_timeout_in_seconds`：流建立后的 chunk 间空闲超时
  - 这意味着当前 Bifrost 的 `stream_idle_timeout` **不是** LiteLLM 的 `stream_timeout` 等价物；它更接近“流中途卡住多久算异常”，而不是“首 token 最晚多久要出来”

- **关键改动**

  - 前端：
    - `ui/app/workspace/providers/fragments/networkFormFragment.tsx`

- **集群、高可用与性能说明**

  - 这轮只改了 Provider 配置页说明文案，没有改 runtime timeout 逻辑
  - 不影响 cluster fanout、自愈、provider hot reload、推理主链、队列或连接池行为
  - 没有新增任何后台轮询、内存缓存或常驻 goroutine，不引入额外性能风险

- **数据表 / Migration / 环境变量**

  - **本轮没有新增数据表**
  - **本轮没有新增 migration**
  - **本轮没有新增环境变量**

- **部署注意**

  - 这轮只涉及 UI 文案增强，按现有前端部署方式更新即可

- **本轮验证**

  - `npm exec next build -- --no-lint` in `ui`
  - `npm exec tsc -- --noEmit` in `ui`  
    说明：当前仓库仍存在既有的 `.next/types` 缺失问题，`tsc` 会报 `TS6053`，不是这轮改动引入的问题

### 2026-04-15 23:34:00 CST | Base Commit 3eb9c7364 | Provider Network：提取 Max Idle Connection Duration 并修正 stale keep-alive 根因

- **问题根因**

  - 这次 provider 提前报错的更底层根因，不只是 request timeout 语义，而是各个 provider client 原来都把 `MaxIdleConnDuration` 写死成了 `30s`
  - 当上游 provider 网关、LB、Ingress 或私有化模型前面的代理更早关闭 idle keep-alive 连接时，Bifrost 仍可能在连接池里复用“看起来还活着、但实际上已被对端关闭”的连接
  - 这类 stale keep-alive 连接在第一次读响应前就被对端断掉时，就会出现：
    - `the server closed connection before returning the first response byte`
    - `failed to execute HTTP request to provider API`

- **本轮完成**

  - `NetworkConfig` 新增：
    - `max_idle_conn_duration_in_seconds`
  - 默认值保持 **30 秒**，保证旧配置向下兼容
  - Provider Network 页面新增 `Max Idle Connection Duration (seconds)`，并和 `Max Connections Per Host` 放在同一块调优窗口中
  - 所有 provider 构造器不再写死 `30s`，统一改为读取 `network_config.max_idle_conn_duration_in_seconds`
  - Bedrock 的 `net/http.Transport.IdleConnTimeout` 也改为同一套配置
  - Provider Network 页的说明文案也同步补充了这层语义：
    - `Request Timeout` 控制整次 upstream call 预算
    - `First Chunk Timeout` 控制首 token / 首 chunk 等待
    - `Stream Idle Timeout` 控制 streaming 过程中 chunk 间静默
    - `Max Idle Connection Duration` 控制 idle keep-alive 连接在池中能存活多久

- **配置建议**

  - 如果上游是内网私有化模型、前面还有 Nginx / Envoy / API Gateway / LB，建议把 `Max Idle Connection Duration` 设得**不高于**上游 keep-alive timeout
  - 如果不确定上游 idle keep-alive 时长，一个更稳的起点通常是：
    - `Max Idle Connection Duration = 5s ~ 15s`
  - 长时间、密集 streaming 推理时，建议继续配合：
    - `Request Timeout = 300s ~ 600s`
    - `First Chunk Timeout = 60s ~ 120s`
    - `Stream Idle Timeout = 60s ~ 120s`

- **关键改动**

  - 后端：
    - `core/schemas/provider.go`
    - `core/providers/openai/openai.go`
    - `core/providers/anthropic/anthropic.go`
    - `core/providers/azure/azure.go`
    - `core/providers/bedrock/bedrock.go`
    - `core/providers/cohere/cohere.go`
    - `core/providers/elevenlabs/elevenlabs.go`
    - `core/providers/fireworks/fireworks.go`
    - `core/providers/gemini/gemini.go`
    - `core/providers/groq/groq.go`
    - `core/providers/huggingface/huggingface.go`
    - `core/providers/mistral/mistral.go`
    - `core/providers/nebius/nebius.go`
    - `core/providers/ollama/ollama.go`
    - `core/providers/openrouter/openrouter.go`
    - `core/providers/parasail/parasail.go`
    - `core/providers/perplexity/perplexity.go`
    - `core/providers/replicate/replicate.go`
    - `core/providers/runway/runway.go`
    - `core/providers/sgl/sgl.go`
    - `core/providers/vllm/vllm.go`
    - `core/providers/vertex/vertex.go`
    - `core/providers/xai/xai.go`
  - 前端：
    - `ui/app/workspace/providers/fragments/networkFormFragment.tsx`
    - `ui/lib/types/config.ts`
    - `ui/lib/constants/config.ts`
    - `ui/lib/types/schemas.ts`
    - `ui/lib/schemas/providerForm.ts`
  - Schema：
    - `transports/config.schema.json`

- **集群、高可用与性能说明**

  - 这轮没有新增后台 worker、轮询器、队列或缓存
  - 没有改普通 inference hot path 的业务路由逻辑，只是把 provider client 的 idle keep-alive 生命周期参数配置化
  - Provider 配置本来就走现有的 ConfigStore + provider hot reload + cluster fanout 机制，所以集群下一致性语义不变
  - 默认值仍然是 30 秒，老配置不需要改就能继续运行

- **数据表 / Migration / 环境变量**

  - **本轮没有新增数据表**
  - **本轮没有新增 migration**
  - **本轮没有新增环境变量**
  - `max_idle_conn_duration_in_seconds` 只是 provider `network_config` 里的新增 JSON 字段，沿用现有 provider 配置存储结构

- **部署注意**

  - 这轮不需要数据库结构变更
  - 如果要启用新配置，直接按现有方式滚动更新前后端即可
  - 集群环境下更新 provider network 配置后，会沿用既有 provider config sync/hot reload 机制下发到其它节点

- **本轮验证**

  - `go test ./schemas ./providers/openai ./providers/anthropic ./providers/azure ./providers/bedrock ./providers/cohere ./providers/elevenlabs ./providers/fireworks ./providers/gemini ./providers/groq ./providers/huggingface ./providers/mistral ./providers/nebius ./providers/ollama ./providers/openrouter ./providers/parasail ./providers/perplexity ./providers/replicate ./providers/runway ./providers/sgl ./providers/vllm ./providers/vertex ./providers/xai -run 'Test(NetworkConfig_StreamIdleTimeoutRoundTrip|ProviderConfig_CheckAndSetDefaultsAppliesMaxIdleConnDuration|DoesNotExist)'` in `core`
  - `go test ./bifrost-http -tags embedui -run TestDoesNotExist` in `transports`
  - `npm exec next build -- --no-lint` in `ui`
  - `npm exec tsc -- --noEmit` in `ui`
