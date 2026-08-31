# locus-link CLI 契约

## 简述

CLI 是 locus-link 当前面向人和 Agent 的命令入口。用户以目标和 capability 发起 Resolve，需要时执行 Safe Probe，再用更新后的 Observation 重新理解路径。

- 声明 YAML、identity 与 Provider data 见[声明契约](声明契约.md)
- 产品语义与名词见[基础核心概念](../base-核心概念.md)
- 内部处理流程见[基础系统设计](../base-系统设计.md)
- 当前实现覆盖见[当前实现快照](../../current-architecture.md)

## 职责

- 定义命令、参数和用户可见结果；
- 定义各命令的副作用；
- 定义 JSON、stdout、stderr 和退出码；
- 保持人和 Agent 使用同一套 CLI 契约。

## 公共稳定性

命令名、command flags、退出码和 `--json` 字段语义是公共契约。内部 Go API、组件拆分、SQLite schema 和外部进程实现不属于公共契约；它们可以随实现演进，但不得静默改变本文件定义的可观察行为。

## 核心操作循环

```text
resolve target + capability
→ 得到显式 Route、Provider 原生上下文、当前证据和文档引用

probe link / route（需要时）
→ 执行 Safe Probe，并追加 Link Observation

resolve again
→ 得到包含最新证据的路径
```

示例：

```text
locus resolve production-host --capability shell --from workstation.dev-a --vantage office-lan
locus probe route.prod-shell --from workstation.dev-a --vantage office-lan
locus resolve production-host --capability shell --from workstation.dev-a --vantage office-lan
```

Resolve 与 Probe 分开：Resolve 读取声明和证据；Probe 测量现实并写入 Observation。Probe success 只表示对应安全检查通过。

## 命令

| 分组 | 命令 | 用途 | 副作用 |
|---|---|---|---|
| Core | `locus resolve <target> --capability <name>` | 解析获得目标 capability 的已知路径 | 无 |
| Core | `locus probe <link-or-route-id>` | 执行 Provider Safe Probe | 追加实际探测 Link 的 Observation |
| Inspection | `locus context` | 查看当前 Scope、Binding、actor、vantage 和 Store | 无 |
| Inspection | `locus show <ref-or-id>` | 查看引用解析后的声明对象 | 无 |
| Inspection | `locus list [entity\|link\|route]` | 列出当前声明 | 无 |
| Inspection | `locus status [link-or-route-id]` | 查看已有 Link 或 Route evidence | 无 |
| Authoring | `locus init --scope-kind <project\|environment> --scope-id <id>` | 创建最小 Registry | 创建目录和 YAML |
| Authoring | `locus validate` | 校验 Registry 声明 | 无 |
| Local UI | `locus web` | 启动只监听本机回环地址的 WebUI | 运行本地 HTTP 服务 |

Inspection 用于查看和排错，不是 Resolve 的前置步骤。Authoring 只维护声明，不要求 actor 或 vantage。`web` 读取同一 Registry/Core，启动时不执行 Probe。

## Resolve

```text
locus resolve <target> --capability <name> [--from <entity>] [--vantage <name>] [--mechanism-bindings <path>]
```

Target 可以是 Project Binding，也可以是可解析的 Entity reference。结果保留输入名称及其 canonical Entity。

候选基数：

- 0 条 Route：`unresolved`；
- 1 条 Route：`resolved`；
- 多条 Route：`ambiguous`，返回全部 candidates。

Candidates 只按 canonical Route ID 稳定排序。Evidence、声明顺序、Provider、Link 数量或最近成功时间都不参与自动选择。

Resolved result 包含：

- 输入名称和 canonical Entity；
- 匹配的显式 Route 和有序 Link；
- 每一步的 Provider、NativeHint 和 Secret reference；
- Entity、Link、Route 关联的 documentation references；
- 当前 Link evidence 和聚合后的 Route evidence。

Resolve 不调用 Probe，也不写 Observation。

## Probe

```text
locus probe <link-or-route-id> [--from <entity>] [--vantage <name>] [--mechanism-bindings <path>] [--timeout <duration>]
```

Link Probe 调用对应 Provider 的 Safe Probe，并追加一条 canonical Link Observation。

Route Probe：

- 按 Route 中的 Link 顺序执行；
- 为每个实际尝试的 Link 追加 Observation；
- 前一步失败后停止；
- 不为未尝试的 Link 写记录；
- 不创建 Route Observation。

Probe failure 是有效测量结果：stdout 仍返回稳定结果，退出码为 `4`，诊断摘要写 stderr。

## Inspection 与 Authoring

### `context`

返回 active Scope、imports、bindings、current Entity、available Provider tools、vantage 和 Store path。

### `show`

只显示声明和引用解析，不返回 Observation。Binding 必须保留输入名称和 canonical target：

```text
input_ref: production-host
ref_type: binding
canonical_target: environment.customer-a::host.prod-01
object: <canonical Entity declaration>
```

### `list`

列出 composed Registry 中的 Entity、Link 或 Route，按 canonical ID 稳定排序。

### `status`

`status <link-id>` 读取当前 Situated Context 下最新的适用 Link Observation。`status <route-id>` 从 constituent Links 的适用 Observation 动态聚合 Route evidence。Status 与 Resolve、Probe 使用相同的 `--from`、`--vantage` 和本地 mechanism bindings。

### `web`

`web` 启动本机 HTTP 服务并提供嵌入式 WebUI。默认使用当前 Registry discovery，可通过 `--registry` 覆盖；`--from`、`--vantage` 和 `--mechanism-bindings` 设置页面的初始 Runtime Context。首版只允许 loopback `--address`，不提供远程服务、认证或远程 Observation Store。

### `init` 与 `validate`

`init` 创建最小本地 Registry，不初始化 Git。`validate` 校验 Scope、imports、bindings、声明引用、Route、Provider 注册，以及 Registry 中显式提供的完整 Provider data，不连接 endpoint。仅由本地 mechanism bindings 提供的 concrete data 在 Resolve、Probe 或 Status 装配现场时校验。

## Registry discovery 与 Runtime Context

`locus` 默认从 cwd 向父目录查找：

```text
.locus/registry/scope.yaml
```

`--registry` 只用于 override、automation、tests 和特殊多 Registry 场景。

- `--from` 表示当前 operational Entity；Resolve 与 Route Probe 用它匹配第一条 Link 的 `from`，后续步骤由显式 Route 顺序和 capability fold 约束；
- `--vantage` 表示 Observation 成立的网络位置，用于隔离 evidence。
- `--mechanism-bindings` 指向 workstation-local YAML；它只覆盖 Link 的 concrete executable 和 `provider_data`，不改变 Link、Route、Binding 或 capability identity。

本地 mechanism bindings 文件使用严格 `locus/v0` YAML：

```yaml
api_version: locus/v0
bindings:
  link.prod-ssh:
    executable: C:/Tools/ssh.exe
    provider_data:
      host: 127.0.0.1
      port: 22022
```

binding key 必须解析为当前 Declared View 中的 Link；文件拒绝未知字段、重复 key、多 document、非 Link 引用和空 binding。`provider_data` 按字段覆盖声明中的稳定默认值，`executable` 覆盖 Provider 默认 executable。该文件属于 Situated Context 输入，不进入 Registry、Graph 或 canonical identity。

当前契约：

- `context`、`resolve`、`probe`、`status` 要求 `--from`；
- 未传 `--vantage` 时使用 host-specific fallback；
- Resolve、Probe 与 Status 通过同一个 Situated Context builder 解析 `from`、vantage 和 mechanism bindings。

## 参数

`--json` 适用于所有返回结构化命令结果的命令；`web` 是长运行服务，不接受 `--json`：

| 命令 | command flags |
|---|---|
| `init` | `--registry --scope-kind --scope-id` |
| `validate` | `--registry` |
| `list` | `--registry` |
| `show` | `--registry` |
| `context` | `--registry --from --vantage --mechanism-bindings` |
| `resolve` | `--registry --from --vantage --mechanism-bindings --capability` |
| `probe` | `--registry --from --vantage --mechanism-bindings --timeout` |
| `status` | `--registry --from --vantage --mechanism-bindings` |
| `web` | `--registry --from --vantage --mechanism-bindings --address` |

无意义的 flag 作为 unknown flag 拒绝，不能静默忽略。

## 输出与退出码

- 正常结果写 stdout，诊断写 stderr；
- `--json` 返回稳定、可脚本化的结构；
- NativeHint 使用 executable 和参数数组，不生成 shell command string；
- Secret value 不进入输出、错误或 Observation；
- 同类对象、candidates、documentation references 和 Provider tools 使用稳定排序。

| 代码 | 含义 |
|---|---|
| `0` | 成功 |
| `1` | 内部错误 |
| `2` | 输入、声明或参数校验错误 |
| `3` | Resolve unresolved 或 ambiguous |
| `4` | Probe 已完成但结果失败；stdout 仍有稳定结果 |

## 测试

CLI 的端到端基准和关键样例统一维护在[测试设计](../测试设计.md)。
