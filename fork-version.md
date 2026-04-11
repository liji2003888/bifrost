# Fork Version Comparison

## 版本定位结论

- 上游当前可以明确视作官方稳定基线的版本是 `transports/v1.4.22`
- `transports/v1.5.0-prerelease2` 是预发布版本，不属于上游正式稳定版
- 当前 fork 更准确的定位是：
  `基于 transports/v1.4.22 的企业定制分支，并选择性吸收了部分 transports/v1.5.0-prerelease2 的能力`

## 版本关系说明

- `transports/v1.4.22`
  - 上游最近一个明确稳定的 transport / HTTP gateway 版本
  - 主要是修复和增强，整体风险较低
- `transports/v1.5.0-prerelease2`
  - 上游下一阶段能力集合的预发布版本
  - 功能增量明显更多，但不应直接视为官方稳定基线

## 对比表

| 功能项 | `v1.4.22` | `v1.5.0-prerelease2` | 当前 fork 状态 |
|---|---|---|---|
| 官方稳定属性 | 是 | 否，预发布 | 不是官方稳定版，是企业定制版 |
| Realtime Support | 无 | 新增 | 已有 |
| Fireworks Provider | 无 | 新增 | 已有 |
| Prompt Repository / Prompts Plugin | 无 | 新增 | 已有，而且已做集群同步 |
| Bedrock Embeddings / Image Gen / Edit / Variation | 无 | 新增 | 已有 |
| Virtual Keys CSV Export | 无 | 新增 | 已有 |
| Path Whitelisting | 无 | 新增 | 已有 |
| Server Bootstrap Timer | 无 | 新增 | 已有 |
| Logging tracking fields | 无 | 新增 | 部分已有，已合入 `user/team/customer`，`businessUnit` 还没有 |
| Per-user OAuth Consent | 无 | 新增 | 部分能力已有，但不是完整 upstream 方案 |
| Access Profiles | 无 | 新增 | 还没有 |
| EnvVar `IsSet` | 无 | 新增 | 还没有 |
| Env-backed value auto-redact 序列化 | 无 | 新增 | 部分已有，但不是按 upstream 这次的完整形态 |
| 一批底层修复包 | 少量 | 较多 | 没有逐项完整对齐 |

## 当前 fork 的实际定位

当前 fork 可以理解为：

- 上游官方稳定基线：`transports/v1.4.22`
- 当前企业 fork：
  `transports/v1.4.22 + 企业定制 + 选择性吸收部分 transports/v1.5.0-prerelease2 能力`

这意味着：

- 功能面已经明显超过 `v1.4.22`
- 但版本稳定性定义不应该直接借用上游 `v1.5.0-prerelease2`
- 更适合使用企业内部自定义版本号来标识当前 fork 的实际可部署状态

## 推荐的版本命名方式

建议不要直接把当前 fork 命名为上游 `v1.5.0`，更适合类似：

- `tcl-bifrost-ee-v1.4.22-r1`
- `tcl-bifrost-ee-v1.4.22+r2`
- `tcl-bifrost-gateway-v1.4.22-enterprise.1`

这样可以同时表达：

- 稳定基线来自 `v1.4.22`
- 企业增强能力来自当前 fork 的独立演进
- 不会和上游官方稳定版本产生歧义
