# Locus Link

Locus Link 为用户和 Agent 提供带运行证据的 operational context：它把 Project、Environment、Binding、Entity、Link 和显式 Route 组合起来，回答“从当前位置如何触及目标，以及这条路径最近是否验证过”。

Locus Link **不执行 Route**。它解析路径、生成 Provider 原生命令上下文，并通过安全检查记录 Link Observation；SSH、FRP、Salt 等原生工具仍负责实际操作。

## 当前能力

- Project / Environment Scope。
- 本地路径 Environment import；namespaced additive composition。
- Project role → canonical Entity Binding。
- Entity、Link、ordered Route YAML 声明。
- FRP、SSH、Salt Provider-native context。
- FRP→SSH `requires/provides` capability composition。
- Link-only Observation、vantage 和 Route evidence 聚合。
- 本机 SQLite Observation Store。
- 面向用户和 Agent 的 JSON CLI。

当前不包含自动路径发现、Planner、Executor、Graph DB、远程 Registry、Web UI 或 Secret Store。

## 构建

需要 Go 1.26 或兼容版本。

```powershell
New-Item -ItemType Directory -Force temp/bin | Out-Null
go build -o temp/bin/locus.exe ./cmd/locus
```

下文用 `locus` 代表构建出的可执行文件。在 PowerShell 中可设置：

```powershell
$locus = "$PWD/temp/bin/locus.exe"
```

## Registry 结构

一个最小 Scope：

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

初始化 Project：

```powershell
& $locus init `
  --scope-kind project `
  --scope-id project.example `
  --registry .locus/registry
```

初始化 Environment 时将 `--scope-kind` 改为 `environment`。

## Scope、Import 与 Binding

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

`customer::host.prod-01` 加载后归一化为：

```text
environment.customer-a::host.prod-01
```

Import 只增加声明，不执行字段合并、override 或 precedence。

## 声明对象

v0 只有 Entity、Link 和 Route 三种声明对象。

### Entity

```yaml
api_version: locus/v0
type: entity
id: host.prod-01
kind: host
name: Customer A Production Host
```

Entity 必须有稳定身份。端口、localhost endpoint、临时 tunnel 和 session 不应建模为 Entity。

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

Link 表示通过具体机制获得 operational reachability/capability。Secret 值不能写入 Registry；这里只保存引用。

### Route

```yaml
api_version: locus/v0
type: route
id: route.prod-shell
steps:
  - link: link.prod-frp
  - link: link.prod-ssh
```

Route 是显式 ordered Link chain：

- target 从最后一个 Link 的 `to` 推导；
- provides 从各 Link 累计推导；
- 前序 `provides` 必须满足后序 `requires`；
- 不要求相邻 Link 满足严格的 `previous.to == next.from`。

## CLI

| 命令 | 用途 |
|---|---|
| `locus init` | 创建最小 Scope Registry |
| `locus validate` | 校验 Scope、imports、bindings 和声明 |
| `locus context` | 查看当前 Scope、imports、Binding 和 Runtime Context |
| `locus list [entity\|link\|route]` | 列出声明对象 |
| `locus show <id>` | 查看对象、canonical identity 和 evidence |
| `locus resolve <target> --capability <name>` | 解析目标、Route 和 Provider-native context |
| `locus check <link-or-route-id>` | 执行 safe probe 并追加 Link Observation |
| `locus status [id]` | 查看 Link evidence 或 Route 聚合状态 |

常用参数：

| 参数 | 用途 |
|---|---|
| `--json` | 输出稳定 JSON |
| `--registry <path>` | 指定 active Registry |
| `--from <entity-ref>` | 指定当前 operational Entity |
| `--vantage <name>` | 指定网络观察位置 |
| `--timeout <duration>` | 设置 Check 超时 |

## 基本工作流

假设当前 Project 声明了 `workstation.dev-a`，并绑定了 `production-host`：

```powershell
$common = @(
  "--registry", ".locus/registry",
  "--from", "workstation.dev-a",
  "--vantage", "office-lan",
  "--json"
)

& $locus validate @common
& $locus context @common
& $locus list route @common
& $locus show production-host @common
& $locus resolve production-host --capability shell @common
```

`resolve` 返回：

- canonical target；
- selected Route；
- 推导出的 capability；
- 每个 Link 的 Provider 和 NativeHint；
- 当前 vantage 下的 evidence status。

它不会启动 FRP、建立 SSH session 或执行 Salt command。

在外部工具已处于预期状态后，可以检查声明路径：

```powershell
& $locus check route.prod-shell @common
& $locus status route.prod-shell @common
& $locus resolve production-host --capability shell @common
```

第二次 Resolve 会使用 Check 写入的 Link Observation，Route evidence 可能从 `unknown` 变为 `success`、`failure` 或 `stale`。

## Observation Store

默认 Store：

- Windows：`%LOCALAPPDATA%/locus-link/state.db`
- Linux/macOS：`$XDG_STATE_HOME/locus-link/state.db`，未设置时为 `~/.local/state/locus-link/state.db`

测试、隔离环境或临时运行可显式设置：

```powershell
$env:LOCUS_STATE_PATH = "$PWD/temp/locus-state.db"
```

Observation 只针对 canonical Link ID，并记录 vantage。Route 状态不单独存储，而是从其 Link Observation 聚合。

## 完整 E2E 案例

可审阅的案例位于：

```text
test/e2e/case/
```

它包含：

- 一个共享 Environment；
- 两个由同一模板物化的 Project；
- Project Binding 和 canonical identity；
- FRP→SSH Route；
- Salt Route；
- 模拟设备状态；
- Context、规范关系和 ordered Route 断言；
- Salt NativeHint 与 Route status 断言；
- success → failure → recovery 的 Observation 闭环；
- 不同 Project/vantage 的 evidence 隔离。

运行：

```powershell
./test/reproduce.ps1
```

运行产物保留在 `temp/e2e-run/`，包括可执行文件、物化 Registry、模拟设备和 SQLite，可用于检查与复现。

完整工作区测试：

```powershell
go test ./...
```

## 设计文档

- [高层设计](documents/design-v0.md)
- [v0 具体设计](documents/v0.md)
