# Locus Link CLI 设计

## 简述

CLI 是 Locus Link 当前面向人和 Agent 的命令入口。用户以目标和 capability 发起 Resolve，需要时执行 Safe Probe，再用更新后的 Observation 重新理解路径。

- 产品语义见 [产品设计](产品设计.md)
- 系统关系见 [系统设计](系统设计.md)
- 当前已实现命令见 [`../current-architecture.md`](../current-architecture.md)

## 职责

- 定义命令、参数和用户可见结果；
- 定义各命令的副作用；
- 定义 JSON、stdout、stderr 和退出码；
- 保持人和 Agent 使用同一套 CLI 契约。

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

Inspection 用于查看和排错，不是 Resolve 的前置步骤。Authoring 只维护声明，不要求 actor 或 vantage。

## Resolve

```text
locus resolve <target> --capability <name> [--from <entity>] [--vantage <name>]
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
locus probe <link-or-route-id> [--from <entity>] [--vantage <name>] [--timeout <duration>]
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

`status <link-id>` 读取指定 vantage 下最新的 Link Observation。`status <route-id>` 从 constituent Link observations 动态聚合 Route evidence。

### `init` 与 `validate`

`init` 创建最小本地 Registry，不初始化 Git。`validate` 校验 Scope、imports、bindings、声明引用、Route 和 Provider data，不连接 endpoint。

## Registry discovery 与 Runtime Context

Locus 默认从 cwd 向父目录查找：

```text
.locus/registry/scope.yaml
```

`--registry` 只用于 override、automation、tests 和特殊多 Registry 场景。

- `--from` 表示当前 operational Entity，用于判断 Route 是否适用于调用位置；
- `--vantage` 表示 Observation 成立的网络位置，用于隔离 evidence。

当前契约：

- `context`、`resolve`、`probe` 要求 `--from`；
- 未传 `--vantage` 时使用 host-specific fallback；
- `status` 只读取 evidence，因此只需要 `--vantage`。

## 参数

`--json` 适用于全部命令：

| 命令 | command flags |
|---|---|
| `init` | `--registry --scope-kind --scope-id` |
| `validate` | `--registry` |
| `list` | `--registry` |
| `show` | `--registry` |
| `context` | `--registry --from --vantage` |
| `resolve` | `--registry --from --vantage --capability` |
| `probe` | `--registry --from --vantage --timeout` |
| `status` | `--registry --vantage` |

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

CLI 的端到端基准和关键样例统一维护在 [测试设计](测试设计.md)。
