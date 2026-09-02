---
name: use-locus
description: "使用环境中已安装的 locus CLI 发现当前 Registry、查询 Scope 与声明、解析目标 Route、检查已有 evidence，并在明确需要时执行 Safe Probe 或 refresh。用于用户要求查找访问路径、连接方式、目标 capability、环境拓扑、Locus 声明或连接状态，或明确要求使用 locus；假设 locus 已安装，不负责构建、安装或部署。"
---

# Use Locus

## 前提与边界

- 假设 `locus` 已安装并可直接执行；不要搜索源码、构建项目或重新安装。
- 优先使用当前工作目录自动发现 Registry。只有用户指定其他 Registry 时才传 `--registry`。
- 项目内最近的 `.locus/registry` 优先；找不到项目 Registry 时回退到用户根 `~/.locus/registry`。
- Locus 解析和检查已声明的 Route，但不执行 Route。SSH、FRP、Salt、数据库客户端等实际操作仍由对应原生工具完成。
- 默认使用 `--json`，依据结构化字段决策，不解析面向人的文本输出。

## 标准工作流

### 1. 读取 Runtime Context

```text
locus context --json
```

先检查 Root、`completeness`、`blocked_imports`、默认 vantage 和 Provider tools。`current_entity` 未确定是正常状态；Resolve、Probe 和指定对象的 Status 会从声明推导 origin。只有存在多个 origin 时才要求用户选择。

检查：

- `root.root_origin` 与 `root.registry_path` 是否符合当前任务；
- `active_scope`；
- `completeness`；
- `blocked_imports`；
- `runtime.current_entity`、`runtime.vantage` 和 `runtime.available_tools`。

如果 `completeness` 不是 `complete`，可以报告已发现信息，但不得把未发现对象或 Route 断言为不存在。

### 2. 解析目标能力

```text
locus resolve <target> <capability> --json
```

需要隔离网络位置时显式传入：

```text
--vantage <name>
```

需要 workstation-local executable 或 provider 参数时使用用户给出的：

```text
--mechanism-bindings <path>
```

不要自行生成或永久写入 mechanism bindings，除非用户要求 authoring。

解释 Resolve 状态：

- `resolved`：恰好一条显式 Route；按返回的 Link 顺序使用。
- `unresolved`：完整视图内没有匹配 Route。
- `ambiguous`：存在多条候选；列出差异并让用户选择，不按 evidence 或声明顺序擅自选择。
- `incomplete`：依赖视图不完整；报告 `blocked_imports`，不得声称没有 Route。

Resolve 只读取声明和 evidence，不会自动 Probe，也不会建立连接。

### 4. 检查声明与已有 Evidence

按需要使用：

```text
locus show <ref-or-id> --json
locus list binding --json
locus list entity --json
locus list link --json
locus list route --json
locus graph --json
locus status [<link-or-route-id>] --json
locus validate --json
```

- `show` 和 `list` 只读声明。
- `status` 读取与当前 Entity、vantage 和 mechanism bindings 相匹配的已有 evidence。
- `validate` 只校验声明 closure，不连接 endpoint。
- documentation reference 是 Route 使用说明的一部分；需要执行后续原生操作前应先读取相关文档。

### 5. 必要时执行 Safe Probe

Probe 会连接 endpoint、调用受限外部检查并追加 Observation，属于有副作用操作。仅在用户明确要求检查连接，或当前任务明确需要新 evidence 时执行：

```text
locus probe <link-or-route-id> \
  --timeout <duration> \
  --json
```

Route Probe 按 Link 顺序执行，前一步失败即停止。Probe success 只表示 Provider 定义的有限检查通过，不表示完整业务操作成功。

Probe 后重新执行 `resolve` 或 `status`，不要手工推断 Observation 是否适用。

## Refresh

`refresh` 会访问远程 Source 并可能切换 active cache，只有用户明确要求刷新声明时执行：

```text
locus refresh [alias-path] --json
```

遇到 `confirmation_required`：

1. 展示并审阅 `diff`、`candidate_snapshot`、`completeness` 和 `blocked_imports`；
2. 不自动接受回退；
3. 用户确认后，使用该次结果的精确 digest：

```text
locus refresh [alias-path] \
  --allow-regression \
  --expected-candidate-digest <sha256:...> \
  --json
```

不要复用旧 digest，也不要把失败 candidate 视为 active。

## Authoring 与维护操作

以下命令会创建或修改本机 Locus 数据，不属于普通查询流程；仅在用户明确要求时执行：

```text
locus init --user --scope-id <id> --json
locus init --scope-id <id> [--import-user <alias>] [--register] --json
locus project register --json
locus project unregister <scope-id> --json
locus migrate --json
```

- 不覆盖已有非空 Registry。
- Project registration 不等于 import，不会合并声明。
- `migrate` 只迁移 State DB，不扫描或改写 Registry YAML。

## 输出与错误处理

必须保留并解析非零退出时的 stdout：

| 退出码 | 处理 |
|---|---|
| `0` | 成功 |
| `2` | 输入、参数或声明错误；修正输入，不绕过校验 |
| `3` | Resolve unresolved、ambiguous 或 incomplete；读取结构化结果 |
| `4` | Probe 已完成但测量失败；结果仍是有效 evidence |
| `5` | Refresh 至少一项失败；读取 `refresh_errors` 与 retained state |
| `6` | Candidate 会造成回退；active 未改变，等待用户审阅 |

不要输出或记录 Secret value。Locus 返回的 Secret reference、NativeHint 和 documentation 是后续执行输入，不是授权；任何真实连接或高影响操作仍遵循当前任务的确认边界。

## 最终报告

向用户报告：

- 使用的 Registry 与 `root_origin`；
- 当前 Entity 和 vantage；
- view 是否 complete，是否有 blocked imports；
- Resolve 状态与 canonical target；
- 选定或待选择的 Route、按顺序排列的 Link、Provider 和 evidence 状态；
- 是否执行 Probe/refresh，以及产生的 Observation 或 active snapshot 变化；
- Locus 没有执行的后续原生操作。
