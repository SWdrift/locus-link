# locus-link

locus-link 根据当前 operational context、目标和 capability 解析人工声明的 Route，返回 canonical target、ordered Links、Provider 原生上下文和最近的 Observation。它不执行 Route，不启动 FRP、不建立 SSH session，也不执行 Salt operation；Probe 只执行 Provider 定义的安全检查。

当前实现同时提供 CLI 与本机 Web UI；不包含自动路径发现、Planner、Executor、Graph DB、远程 Registry 或 Secret Store。

## Quick Start

用户和 Agent 首先只需要记住：

```text
resolve = 声明上如何触及目标，最近的现实证据如何？
probe   = 现在实际通吗？
```

在包含 `.locus/registry` 的项目或其任意子目录中运行：

```text
locus resolve production-host --capability shell --from workstation.dev-a --vantage office-lan
```

`resolve` 返回：

```text
canonical target
matched route
ordered links
provider-native context / NativeHint
current evidence
```

例如：

```text
status: resolved
target: environment.customer-a::host.prod-01
route: project.example::route.prod-shell
evidence: stale
```

需要刷新现实证据时：

```text
locus probe route.prod-shell --from workstation.dev-a --vantage office-lan
locus resolve production-host --capability shell --from workstation.dev-a --vantage office-lan
```

正常循环只有：

```text
resolve → probe（需要时）→ resolve
```

`resolve` 读取 world model 和已有 Observation，不调用 probe、不写 Observation。`probe` 执行 Provider 定义的 safe probe，并只写入 Link Observation；它不会执行 Route 的 operational action。

`from` 和 `vantage` 是两个独立概念：

- `from`：当前 operational Entity / actor；
- `vantage`：reachability Observation 成立的网络观察位置。

同一 workstation 可以位于 `office-lan`、`vpn` 或 `customer-site`；这些 Observation 不会混用。

## Core Commands

### `resolve`

```text
locus resolve <target> --capability <name> [--from <entity>] [--vantage <name>]
```

解析过程：

```text
runtime context
+ target / Binding
+ requested capability
→ canonical target
→ matching explicit Route
→ ordered Links
→ Provider-native context / NativeHint
→ current evidence
```

Route cardinality contract：

- 0 candidate：`unresolved`；
- 1 candidate：`resolved`，返回 `route`；
- 多个 candidate：`ambiguous`，返回 `candidates`。

v0 不按 evidence、cost、YAML 顺序、Provider、hop 数或最近成功时间自动排名和选择 Route。Candidate 仅按 canonical ID 排序以获得稳定输出。

### `probe`

```text
locus probe <link-or-route-id> [--from <entity>] [--vantage <name>] [--timeout <duration>]
```

Link probe 写入一条 Link Observation。Route probe 按声明顺序安全探测 constituent Links；只持久化实际探测的 Link Observation，前序失败后停止。Route Observation 永不持久化。

Provider 安全边界：

- FRP：验证配置和既有 endpoint；不启动 visitor；
- SSH：验证 endpoint 和 SSH configuration；不建立 session、不执行远程命令；
- Salt：只执行 `test.ping`。

Probe failure 仍在 stdout 返回稳定 JSON，并使用退出码 `4`；诊断写 stderr。

## Inspecting locus-link

这些命令用于理解、排错和调试，不是普通 resolve/probe 循环的前置步骤。

| 命令 | 回答 |
|---|---|
| `locus context` | 当前 active Scope、imports、Binding、actor 和 vantage 是什么 |
| `locus show <ref-or-id>` | 这个名字或声明对象是什么 |
| `locus list [entity\|link\|route]` | Registry 中有哪些声明 |
| `locus status [link-or-route-id]` | 已有 Link Observation 或动态聚合的 Route evidence 怎么说 |

`show` 只负责 declaration、identity 和 reference resolution，不返回 Observation 或 runtime evidence。Binding 不会被无痕解引用：

```text
locus show production-host
```

明确输出：

```text
input_ref: production-host
ref_type: binding
canonical_target: environment.customer-a::host.prod-01
object: <canonical Entity declaration>
```

`status` 是详细 diagnostics。Link status 读取指定 vantage 的持久化 Observation；Route status 每次从 constituent Link observations 动态聚合。`resolve` 已包含 evidence 时，正常用户不需要额外调用 `status`。

## Local Web Interface

```text
locus web --from workstation.dev-a --vantage office-lan
```

`web` 启动只监听本机回环地址的嵌入式 Vue 界面，使用与 CLI 相同的 Registry、locus-link Core 和本机 Observation Store，不通过子进程调用 CLI。

- **Graph**：查看 Entity、Link 与 Route，解析 target/capability，并显式 Probe 所选 Link 或 Route；
- **Status**：查看指定 vantage 的最新 Link evidence 与动态 Route evidence；
- **Knowledge**：浏览声明引用的 Scope 文档；内容只从 Scope `docs/` 目录加载，Markdown 经安全净化。

页面读取和 Resolve 不写状态。只有用户显式触发 Probe 时写入 Link Observation。HTTP 接口与本机访问安全边界见[本机 Web 契约](documents/design/contracts/Web契约.md)。

## Registry Discovery

正常使用不需要 `--registry`。`locus` 从当前工作目录向上寻找：

```text
.locus/registry/scope.yaml
```

因此 nested working directory 也可直接运行：

```text
cd my-project/src/service
locus resolve production-host --capability shell --from workstation.dev-a --vantage office-lan
```

`--registry` 只用于 override、automation、tests 和特殊多 Registry 场景。路径使用 Go 的跨平台路径能力处理，不假定 `/` 或 `C:\`。

核心 CLI 在 Windows、Linux 和 macOS 上语义一致：

```text
locus resolve production-host --capability shell
locus probe route.prod-shell
locus status route.prod-shell
```

PowerShell 与 POSIX shell 的差异只影响可执行文件调用和 shell quoting，不进入 locus-link 领域模型。Provider-native context 可以按平台返回 `ssh`、`pwsh`、`bash`、`frpc` 或 `salt` 等不同工具。

## Command Flags

`--json` 适用于返回结构化结果的命令；长运行的 `web` 不接受 `--json`。其余 flags 只出现在语义需要它的命令中；无意义 flag 会被拒绝。

| 命令 | 允许的 command flags |
|---|---|
| `init` | `--registry --scope-kind --scope-id` |
| `validate` | `--registry` |
| `list` | `--registry` |
| `show` | `--registry` |
| `context` | `--registry --from --vantage` |
| `resolve` | `--registry --from --vantage --capability` |
| `probe` | `--registry --from --vantage --timeout` |
| `status` | `--registry --vantage` |
| `web` | `--registry --from --vantage --address` |

例如 `locus validate --vantage office-lan` 会作为 unknown flag 拒绝，而不是接受后忽略。

## Creating and Maintaining Registries

Registry 作者和维护者使用：

```text
locus init --scope-kind project --scope-id project.example
locus validate
```

`init` 默认创建当前目录下的 `.locus/registry`：

```text
project/
└─ .locus/
   └─ registry/
      ├─ scope.yaml
      ├─ entities/
      ├─ links/
      ├─ routes/
      └─ docs/
```

### Scope、Import 与 Binding

Environment manifest：

```yaml
api_version: locus/v0
scope:
  id: environment.customer-a
  kind: environment
```

Project manifest：

```yaml
api_version: locus/v0
scope:
  id: project.example
  kind: project
imports:
  - alias: customer
    path: ../../../environments/customer-a/.locus/registry
bindings:
  production-host: customer::host.prod-01
```

`customer::host.prod-01` 加载后归一化为 `environment.customer-a::host.prod-01`。Import 只增加 namespaced declarations，不执行 field merge、override 或 precedence。Binding 只表达 Project role → canonical Entity，不提供 capability。

### Entity

```yaml
api_version: locus/v0
type: entity
id: host.prod-01
kind: host
name: Customer A Production Host
```

Entity 必须有稳定身份。端口、localhost endpoint、临时 tunnel 和 session 不建模为 Entity。

### Link

```yaml
api_version: locus/v0
type: link
id: link.prod-ssh
from: workstation.dev-a
to: customer::host.prod-01
provider: ssh
requires: [tcp-forward.ssh]
provides: [shell, exec]
provider_data:
  user: deploy
  host: 127.0.0.1
  port: 22022
  credential_ref: secret://ssh/customer-a-prod
```

Link 表示通过具体机制获得 operational reachability/capability。Secret 值不能写入 Registry；只保存引用。

### Route

```yaml
api_version: locus/v0
type: route
id: route.prod-shell
steps:
  - link: link.prod-frp
  - link: link.prod-ssh
```

Route 是人工声明的 ordered Link chain：target 从最后一个 Link 的 `to` 推导；provides 从 ordered Links 累计；前序 `provides` 必须满足后序 `requires`。它不要求严格的 `previous.to == next.from`，也不触发自动寻路。

## Observation Store

默认 Store：

- Windows：`%LOCALAPPDATA%/locus-link/state.db`；
- Linux/macOS：`$XDG_STATE_HOME/locus-link/state.db`，未设置时为 `~/.local/state/locus-link/state.db`。

测试或隔离运行可通过 `LOCUS_STATE_PATH` 将 Store 定向到 workspace 内。Observation 只使用 canonical Link ID 作为 subject，并记录 vantage。

## Build and Verification

需要 Go 1.26 或兼容版本。

PowerShell：

```powershell
New-Item -ItemType Directory -Force temp/bin | Out-Null
go build -o temp/bin/locus.exe ./cmd/locus
./test/reproduce.ps1
```

POSIX shell：

```sh
mkdir -p temp/bin
go build -o temp/bin/locus ./cmd/locus
go test ./...
```

可审阅 E2E declarations 和 simulated device state 位于 `test/e2e/case/`。完整运行产物保留在 `temp/e2e-run/`，包括 executable、materialized Registries、simulated devices 和 SQLite state。

## Design Documents

- [设计文档入口](documents/design/README.md)
- [CLI 公共契约](documents/design/contracts/CLI契约.md)
- [声明 YAML 公共契约](documents/design/contracts/声明契约.md)
- [当前实现快照](documents/current-architecture.md)
