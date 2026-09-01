# 本机 Web 契约

本契约定义 `locus web` 的本机 HTTP 界面和 `/api/v0` 可观察行为。领域语义继承[声明契约](声明契约.md)与[CLI 契约](CLI契约.md)；Web 与 CLI 共用同一 Registry、Core 和本机 Observation Store。

## 启动与安全边界

```text
locus web [--registry <path>] [--from <entity>] [--vantage <name>] [--address <loopback:port>]
```

- 默认地址为 `127.0.0.1:7070`；只接受 IPv4/IPv6 loopback listener。
- 服务拒绝非本机 `Host` 和 cross-site fetch，返回限制脚本、对象、frame 与外连的 CSP。
- `--from`、`--vantage` 和 `--mechanism-bindings` 提供页面初始 Situated Context；UI 的每次 Resolve、Status 或 Probe 请求显式携带 `from` 与 vantage，服务端统一应用启动时装载的本地 mechanism bindings。
- UI 和 API 不返回 Provider `provider_data`、Secret 或环境凭据。

## 读取投影

| 方法与路径 | 行为 |
|---|---|
| `GET /api/v0/locus/scopes` | 用户根与项目反向登记目录；返回 `kind`、Registry path、availability、openable 与当前 active 标记 |
| `GET /api/v0/locus/dependencies?root=<scope-id>` | 指定可打开 Root Scope 的 Dependency Snapshot；节点按 Scope identity 展示，保留多 alias path、blocked edge、cycle、digest 与 snapshot digest |
| `GET /api/v0/context[?scope=<scope-id>]` | Scope、root discovery/registration/cache、imports、完整 Bindings、Observation Store 和初始 runtime context |
| `GET /api/v0/graph[?scope=<scope-id>]` | Scope、Binding、Entity、Link、Route 的确定性声明投影；不包含 Provider data |
| `GET /api/v0/status?scope=<scope-id>[&from=<entity>]&vantage=<name>` | 当前 Situated Context 下适用的最新 Link evidence 与动态 Route evidence；没有 current Entity 的空 Scope 仍返回可读取的空状态 |
| `GET /api/v0/knowledge[?scope=<scope-id>]` | 经声明引用并去重的文档索引 |
| `GET /api/v0/knowledge/{id}[?scope=<scope-id>]` | 文档元数据与正文；只读取所属 Scope 的 `docs/` 内文件 |
| `GET /api/v0/validate[?scope=<scope-id>]` | 当前 Registry 的真实 complete/partial 校验状态、对象计数与 blocked import diagnostics |
| `GET /api/v0/resolve?scope=<scope-id>&target=<ref>&capability=<name>&from=<entity>&vantage=<name>` | 与 CLI `resolve` 相同的只读解析结果 |

Graph 中 Entity 是节点、Link 是有向边、Route 是有序 Link overlay；Scope 与 Binding 是上下文和注解，不伪装成 operational 节点。数组按 canonical ID 或稳定键排序；没有元素的集合编码为 `[]`，不得编码为 `null`。文档 ID 由 Scope 和 Scope 内相对路径稳定派生；同一文件被多个声明引用时只返回一个文档，并保留所有 associations。

Status 的 `provider` 来自 Link 声明，与 Observation 是否存在无关；`unknown` 和“从未”只表示当前完整 applicability 下没有匹配的 Observation。只有显式 Probe 才能把 Link evidence 推进为 `success` 或 `failure`。

Context、Graph、Status、Validate、Resolve 和 Probe 的读取结果统一携带适用的 `completeness` 与 `blocked_imports`。UI 必须显式呈现 partial，不得把已加载的部分声明投影伪装为完整视图。

## Dependency refresh

```http
POST /api/v0/locus/refresh
Content-Type: application/json

{"scope_id":"project.alpha","alias_path":"","allow_regression":false,"expected_candidate_digest":""}
```

请求显式获取 remote Sources，但先在隔离 candidate 中构建完整 Dependency Snapshot，并返回 active/candidate diff。Candidate 新增 blocked edge、cycle、authority conflict 或使 completeness 回退时返回 `status: "confirmation_required"`，active pointer 保持不变。用户确认时再次提交 `allow_regression: true` 和刚审阅的 `expected_candidate_digest`；candidate 已变化则拒绝激活并要求重新审阅。通过确认后，所有涉及的 edge cache pointer 与无冲突 Scope authority 在单个 SQLite transaction 中切换。

## Probe

```http
POST /api/v0/probes
Content-Type: application/json

{"subject":"route.prod-shell","from":"workstation.dev-a","vantage":"office-lan","timeout_ms":10000}
```

Probe 语义、Provider 安全边界和 Observation 写入规则与 CLI `probe` 相同。`subject` 必填；`timeout_ms` 取值 `1..60000`。请求体最大 64 KiB，未知字段被拒绝。Probe 完成且 safe probe 失败时仍返回 `200` 和 `status: "failure"`；输入错误返回 `400`，Store 或内部执行错误返回 `500`。

## UI 行为

- **Locus / Scope Catalog**：始终可用，展示用户根与项目登记、availability，并通过显式“打开”进入 Scope 工作区。
- **Locus / Dependency Graph**：默认合并 Catalog 中全部可打开 Root 的完整 Dependency Snapshot，并按 Scope identity 去重；Root 选择、搜索与“仅问题”只从该总图派生子图。节点选中只更新 provenance 与 blocked/cycle diagnostics，只有显式“打开 Scope”才改变工作区。
- **Graph**：查看当前 Scope 的声明图、选择 Route/Link、Resolve，并显式触发所选对象的 Probe。
- **Status**：按 current Entity 与 vantage 查看适用的 Link evidence 和每次读取时聚合的 Route 状态。
- **Knowledge**：按声明 association 浏览 Scope 文档；Markdown 禁用内嵌 HTML并在渲染后净化。
- **Inspect**：只读查看 Context、Validate、Scope/Import/Binding/Source provenance，并统一检索 Binding、Entity、Link 与 Route 声明。
- Locus 页面是稳定全局入口；Graph、Status、Knowledge、Inspect 属于 `/scopes/:scope-id/*` 工作区。Remote/cache Scope 可选中和查看，但未登记时不可独立打开。
- Scope 工作区各视图通过 canonical object ID 或稳定 document ID 互相跳转；技术标识、路径、NativeHint 和 diagnostics 保持原值并可复制。
- Refresh 的普通成功、部分成功与失败通过不参与布局的浮层消息反馈；需要用户审阅并确认回退时使用模态 diff，不在图上方插入临时 Alert。

Probe 是唯一写 Observation 的 Web 操作。页面加载、Graph、Status、Knowledge、Inspect、Validate 和 Resolve 均不得隐式 Probe 或写状态。

## 兼容与错误

`/api/v0` 响应使用 JSON；成功响应字段保持向后兼容，新增可选字段允许兼容增加。未知 API 路径不属于 SPA fallback。API 错误使用对应 HTTP 状态并返回：

```json
{"error":"diagnostic message"}
```

破坏字段类型、字段语义、Probe 副作用或安全边界需要新的 API version。前端组件、CSS、Go handler 和 SQLite schema 不属于公共契约。
