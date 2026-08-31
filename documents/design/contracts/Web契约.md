# 本机 Web 契约

本契约定义 `locus web` 的本机 HTTP 界面和 `/api/v0` 可观察行为。领域语义继承[声明契约](声明契约.md)与[CLI 契约](CLI契约.md)；Web 与 CLI 共用同一 Registry、Core 和本机 Observation Store。

## 启动与安全边界

```text
locus web [--registry <path>] [--from <entity>] [--vantage <name>] [--address <loopback:port>]
```

- 默认地址为 `127.0.0.1:7070`；只接受 IPv4/IPv6 loopback listener。
- 服务拒绝非本机 `Host` 和 cross-site fetch，返回限制脚本、对象、frame 与外连的 CSP。
- `--from`、`--vantage` 只提供页面初始 context；每次 Resolve、Status 或 Probe 请求显式携带当前值。
- UI 和 API 不返回 Provider `provider_data`、Secret 或环境凭据。

## 读取投影

| 方法与路径 | 行为 |
|---|---|
| `GET /api/v0/context` | active Scope、imports、Bindings 和初始 runtime context |
| `GET /api/v0/graph` | Scope、Binding、Entity、Link、Route 的确定性声明投影；不包含 Provider data |
| `GET /api/v0/status?vantage=<name>` | 指定 vantage 的最新 Link evidence 与动态 Route evidence |
| `GET /api/v0/knowledge` | 经声明引用并去重的文档索引 |
| `GET /api/v0/knowledge/{id}` | 文档元数据与正文；只读取所属 Scope 的 `docs/` 内文件 |
| `GET /api/v0/validate` | 当前 Registry 的静态校验结果与对象计数 |
| `GET /api/v0/resolve?target=<ref>&capability=<name>&from=<entity>&vantage=<name>` | 与 CLI `resolve` 相同的只读解析结果 |

Graph 中 Entity 是节点、Link 是有向边、Route 是有序 Link overlay；Scope 与 Binding 是上下文和注解，不伪装成 operational 节点。数组按 canonical ID 或稳定键排序。文档 ID 由 Scope 和 Scope 内相对路径稳定派生；同一文件被多个声明引用时只返回一个文档，并保留所有 associations。

## Probe

```http
POST /api/v0/probes
Content-Type: application/json

{"subject":"route.prod-shell","from":"workstation.dev-a","vantage":"office-lan","timeout_ms":10000}
```

Probe 语义、Provider 安全边界和 Observation 写入规则与 CLI `probe` 相同。`subject` 必填；`timeout_ms` 取值 `1..60000`。请求体最大 64 KiB，未知字段被拒绝。Probe 完成且 safe probe 失败时仍返回 `200` 和 `status: "failure"`；输入错误返回 `400`，Store 或内部执行错误返回 `500`。

## UI 行为

- **Graph**：查看声明图、选择 Route/Link、Resolve，并显式触发所选对象的 Probe。
- **Status**：按 vantage 查看 Link 最新证据和每次读取时聚合的 Route 状态。
- **Knowledge**：按声明 association 浏览 Scope 文档；Markdown 禁用内嵌 HTML并在渲染后净化。

Probe 是唯一写 Observation 的 Web 操作。页面加载、Graph、Status、Knowledge、Validate 和 Resolve 均不得隐式 Probe 或写状态。

## 兼容与错误

`/api/v0` 响应使用 JSON；成功响应字段保持向后兼容，新增可选字段允许兼容增加。未知 API 路径不属于 SPA fallback。API 错误使用对应 HTTP 状态并返回：

```json
{"error":"diagnostic message"}
```

破坏字段类型、字段语义、Probe 副作用或安全边界需要新的 API version。前端组件、CSS、Go handler 和 SQLite schema 不属于公共契约。
