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
| Inspection | `locus context` | 查看根来源、Scope graph、registration、cache 与 Runtime Context | 无 |
| Inspection | `locus graph` | 查看 composed Declared View 与 blocked imports | 无 |
| Inspection | `locus show <ref-or-id>` | 查看引用解析后的声明对象 | 无 |
| Inspection | `locus list [binding\|entity\|link\|route]` | 列出当前声明 | 无 |
| Inspection | `locus status [link-or-route-id]` | 查看已有 Link 或 Route evidence | 无 |
| Inspection | `locus version` | 查看应用、上一版本、State schema、commit 与 artifact kind | 无 |
| Authoring | `locus user init --scope-id <id>` | 创建用户根 Registry | 创建目录和 YAML |
| Authoring | `locus init --scope-id <id>` | 创建通用 Scope Registry | 创建目录和 YAML，可选 registration |
| Authoring | `locus project register|unregister|list` | 管理项目反向登记 | 只修改用户级 State DB |
| Authoring | `locus refresh [alias-path]` | 显式刷新 remote imports | 获取、校验并原子激活 cache |
| Authoring | `locus validate` | 校验 Registry 声明与 closure | 无 |
| Authoring | `locus migrate` | 将上一版本本机 State 迁移到当前 schema | 备份并事务迁移 State DB |
| Local UI | `locus web` | 启动只监听本机回环地址的 WebUI | 运行本地 HTTP 服务 |

Inspection 用于查看和排错，不是 Resolve 的前置步骤。Authoring 不要求 actor 或 vantage。除显式 `refresh` 外，`graph/list/show/context/resolve/status/probe` 均不得 fetch 或联网。

## Resolve

```text
locus resolve <target> --capability <name> [--from <entity>] [--vantage <name>] [--mechanism-bindings <path>]
```

Target 可以是根 Scope Binding，也可以是可解析的 Entity reference。结果保留输入名称及其 canonical Entity。

Complete view 的候选基数：

- 0 条 Route：`unresolved`；
- 1 条 Route：`resolved`；
- 多条 Route：`ambiguous`，返回全部 candidates。

Partial view 无论已发现 0、1 或多条候选，都返回 `status: incomplete`、全部已发现 candidates、无唯一 `route`，退出码 `3`。Complete view 中未知 target 是输入错误；partial view 中未知 target 返回 incomplete，不声称对象不存在。

Candidates 只按 canonical Route ID 稳定排序。Evidence、声明顺序、Provider、Link 数量或最近成功时间都不参与自动选择。

Resolved result 包含：

- 输入名称和 canonical Entity；
- 匹配的显式 Route 和有序 Link；
- 每一步的 Provider、NativeHint 和 Secret reference；
- Entity、Link、Route 关联的 documentation references；
- 当前 Link evidence 和聚合后的 Route evidence；
- 顶层 `completeness` 与 `blocked_imports`。

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

Partial view 中，只允许 Probe 最终 view 内已完整加载并校验的 Link/Route。Blocked 或 unavailable 引用是输入错误，Observation 数量不变。结果仍携带顶层 `completeness` 与 `blocked_imports`。Probe failure 是有效测量结果：stdout 仍返回稳定结果，退出码为 `4`。

## Inspection 与 Authoring

### `context`

返回 `root_origin: explicit|project|user`、根 Scope、用户 Registry、imports、bindings、项目 registration、active Source revision/digest、last refresh diagnostics、current Entity、available Provider tools、vantage 和 Store path。

### `graph`

返回按 canonical ID/alias path 稳定排序的 scopes、import edges、alias paths、bindings、entities、links、routes、Source provenance、`completeness` 和 `blocked_imports`。

### `show`、`list` 与 `status`

`show` 只显示声明和引用解析，不返回 Observation。Binding 保留 input ref、canonical Binding ID 和 canonical target。`list` 支持 `binding|entity|link|route`。`status` 只聚合已加载对象的 evidence。三者都返回顶层 view diagnostics；`show` 引用 blocked object 时为输入错误。

### `version` 与 `migrate`

`locus version --json` 返回 `version`、`previous_version`、`state_schema_version`、`commit` 与 `artifact`。版本查询不发现 Registry、不打开 State，也不读取运行现场。

`locus migrate [--state <path>] [--backup-dir <path>]` 只处理 locus-link 本机 State DB。空数据库初始化为当前 schema；上一 schema 先生成一致备份再事务迁移；当前 schema 返回 `no_change`；其他版本拒绝。JSON 返回 `status: initialized|migrated|no_change`、`state_path`、`backup_path`、`from_version` 与 `to_version`。该命令不扫描或改写用户及项目 Registry。

### `user init`、`init` 与 `project`

- `locus user init --scope-id <id>` 创建 `<LOCUS_HOME>/registry` 的最小 `locus/v1` Registry；已存在非空 Registry 时拒绝覆盖。
- `locus init --scope-id <id> [--registry <path>] [--import-user <alias>] [--register]` 创建通用 Scope。`--import-user` 写入 expected user `scope_id` 和 `${LOCUS_HOME}/registry` directory Source；`--register` 只在完整校验后登记。
- `locus project register [--registry <path>]`、`project unregister <scope-id>`、`project list` 只管理反向登记，不改变 imports 或合并声明。
- `--scope-kind` 不存在。

### `refresh`

`locus refresh [alias-path] [--registry] [--allow-regression --expected-candidate-digest <sha256:...>]` 无参数时刷新根 closure 的全部 remote imports；参数按 normalized alias path 只刷新目标 edge。获取先写入隔离 candidate，再以 overlay cache 构建完整 Dependency Snapshot，并与 active snapshot 生成稳定 diff。JSON 返回 `status: success|partial|failure|confirmation_required`、`active_snapshot`、`candidate_snapshot`、`diff`、`activated`、`retained`、`refresh_errors`、`completeness` 与 `blocked_imports`。

Candidate 自身无效时禁止激活；新增 blocked import、cycle、authority conflict 或 completeness 回退时保持 active 不变并退出 `6`。调用方审阅 snapshot 后必须同时提交 `--allow-regression` 与该结果的 `--expected-candidate-digest`；candidate 已变化或缺少 digest 时拒绝激活。涉及的 edge cache 与无冲突 Scope authority 在单个 SQLite transaction 中切换。一项或多项获取或校验失败仍输出结果并退出 `5`；输入或声明错误退出 `2`。

### `validate` 与 `web`

`validate` 校验根与已加载 closure，不连接 endpoint、不执行 Provider。Partial 时返回结构化 diagnostics 并退出 `2`。`web` 继续复用同一 Core 和 loopback 边界，本轮不改变 Web 公共契约。

## Registry discovery 与 Runtime Context

根选择优先级：

1. 显式 `--registry`，`root_origin: explicit`；
2. 从 cwd 向上发现 `.locus/registry/scope.yaml`，`root_origin: project`；
3. `${LOCUS_HOME}/registry`，`root_origin: user`。

项目 registration 不参与 declaration merge。项目根 context 同时报告其 `scope_id` 是否登记，以及是否显式 import 用户 Registry。

- `--from` 表示当前 operational Entity；Resolve 与 Route Probe 用它匹配第一条 Link 的 `from`，后续步骤由显式 Route 顺序和 capability fold 约束；
- `--vantage` 表示 Observation 成立的网络位置，用于隔离 evidence；
- `--mechanism-bindings` 指向 workstation-local YAML；它只覆盖 Link 的 concrete executable 和 `provider_data`，不改变 Link、Route、Binding 或 capability identity。

本地 mechanism bindings 文件使用严格 `locus/v1` YAML：

```yaml
api_version: locus/v1
bindings:
  link.prod-ssh:
    executable: C:/Tools/ssh.exe
    provider_data:
      host: 127.0.0.1
      port: 22022
```

binding key 必须解析为当前 Declared View 中的 Link；文件拒绝未知字段、重复 key、多 document、非 Link 引用和空 binding。`provider_data` 按字段覆盖声明中的稳定默认值，`executable` 覆盖 Provider 默认 executable。该文件属于 Situated Context 输入，不进入 Registry、Graph 或 canonical identity。

`context`、`resolve`、`probe`、`status` 要求 `--from`；未传 `--vantage` 时使用 host-specific fallback；四个命令通过同一个 Situated Context builder 解析现场输入。

## 参数

`--json` 适用于所有返回结构化命令结果的命令；`web` 是长运行服务，不接受 `--json`：

| 命令 | command flags |
|---|---|
| `user init` | `--scope-id` |
| `init` | `--registry --scope-id --import-user --register` |
| `project register` | `--registry` |
| `project unregister`、`project list` | 无 |
| `version` | 无 |
| `migrate` | `--state --backup-dir` |
| `refresh` | `--registry --allow-regression --expected-candidate-digest` |
| `validate`、`graph`、`list`、`show` | `--registry` |
| `context` | `--registry --from --vantage --mechanism-bindings` |
| `resolve` | `--registry --from --vantage --mechanism-bindings --capability` |
| `probe` | `--registry --from --vantage --mechanism-bindings --timeout` |
| `status` | `--registry --from --vantage --mechanism-bindings` |
| `web` | `--registry --from --vantage --mechanism-bindings --address` |

无意义的 flag 作为 unknown flag 拒绝，不能静默忽略。

## 输出与退出码

- 正常结果写 stdout，诊断写 stderr；
- 所有 Declared View 命令的 JSON 顶层统一携带 `completeness` 与 `blocked_imports`，不创建命令私有 diagnostics；
- `--json` 返回稳定、可脚本化的结构；
- NativeHint 使用 executable 和参数数组，不生成 shell command string；
- Secret value、URI userinfo、外部进程 raw stdout/stderr 和 HTTP body 不进入输出、错误、缓存 metadata 或 Observation；
- 同类对象、candidates、documentation references、alias paths 和 Provider tools 使用稳定排序。

| 代码 | 含义 |
|---|---|
| `0` | 成功 |
| `1` | 内部错误 |
| `2` | 输入、声明、参数校验错误，或 Validate partial |
| `3` | Resolve unresolved、ambiguous 或 incomplete |
| `4` | Probe 已完成但结果失败；stdout 仍有稳定结果 |
| `5` | Refresh 至少一项获取或校验失败；stdout 仍有结构化结果 |
| `6` | Refresh candidate 会使依赖视图回退，active 保持不变；stdout 携带快照与 diff |

## 测试

CLI 的端到端基准和关键样例统一维护在[测试设计](../测试设计.md)。
