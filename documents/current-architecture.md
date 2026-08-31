# Locus Link 当前实现快照

本文只记录当前 Go 实现与可执行 E2E 已经证明的事实，作为易变实现清单。公共目标由 [`design/contracts/`](design/contracts/README.md) 定义，Workspace 目标由 [Workspace 与 Registry 设计](design/Workspace与Registry设计.md)定义，Core 内部目标由[系统设计](design/系统设计.md)和[数据流与存储设计](design/数据流与存储设计.md)定义。除“明确未实现”一节外，本文不描述计划中的架构。

## 1. 总体结构与数据流

当前发布一个 `locus` executable：Cobra CLI 与 loopback WebUI 共用 `internal/locus` Core。声明保存在 YAML Registry 中，运行观测追加到本机 SQLite；Resolve 读取声明和既有 Observation，但不会主动 Probe。

```mermaid
flowchart LR
    User["用户 / Agent"] --> CLI["locus CLI / local Web<br/>init · validate · context · list · show<br/>resolve · probe · status · web"]

    CWD["当前目录或 --registry"] --> Discover["向上发现<br/>.locus/registry/scope.yaml"]
    Discover --> Loader["严格 YAML 加载器"]
    YAML["scope.yaml<br/>entities/*.yaml<br/>links/*.yaml<br/>routes/*.yaml"] --> Loader
    Imports["项目 Scope 的<br/>Environment imports"] --> Loader
    Loader --> Registry["内存 Registry<br/>Scope · Binding · Entity · Link · Route"]

    Flags["--from · --vantage<br/>PATH 可用工具"] --> Runtime["RuntimeContext"]
    Registry --> Resolve["Resolve<br/>筛选显式 Route"]
    Runtime --> Resolve
    Providers["Provider registry<br/>frp-stcp · ssh · salt"] --> Resolve
    SQLite[("SQLite<br/>observations")] -->|"Latest(subject, vantage)"| Resolve
    Resolve --> Result["resolved / unresolved / ambiguous<br/>NativeHint + Evidence"]

    Registry --> Probe["Probe Link 或 Route"]
    Runtime --> Probe
    Providers --> Probe
    Probe -->|"执行 Provider safe probe"| Tools["frpc / ssh / salt<br/>及 TCP endpoint"]
    Probe -->|"Append Observation"| SQLite
    SQLite --> Status["status"]
    Registry --> Status
```

## 2. 声明与存储布局

### 2.1 Registry 目录

```text
.locus/registry/
├── scope.yaml
├── entities/*.yaml
├── links/*.yaml
├── routes/*.yaml
└── docs/                 # init 会创建；当前加载器不读取
```

- 未显式传入 `--registry` 时，从当前工作目录逐级向上查找 `.locus/registry/scope.yaml`。
- `scope.yaml` 必须声明 `api_version: locus/v0`，Scope `kind` 只接受 `project` 或 `environment`。
- 对象文件只读取上述三个一级目录中的 `.yaml`/`.yml` 普通文件，不递归目录；每个文件只允许一个 YAML document。
- YAML 解码拒绝未知字段、重复 key 和多 document。当前没有独立 JSON Schema；Go struct 与严格 YAML 解码器构成实际可执行 schema。
- Entity 目前承载 `id/kind/name/labels/documentation`；Link 承载 `from/to/provider/requires/provides/provider_data/documentation`；Route 仅含有序 Link steps 与 `documentation`。`provider_data` 是 Provider 自行解释的开放 map。
- 对象也有 `api_version` 字段，但当前加载器没有校验其值；`type` 必须能分派为 `entity`、`link` 或 `route`。

### 2.2 Scope、Import、Binding 与身份

- 规范身份固定为 `<scope-id>::<local-id>`。同一 Scope 内 Entity、Link、Route 共用 local ID 名字空间，重复即加载失败；不同 Scope 可复用 local ID。
- 不带 `::` 的引用按声明对象所在 Scope 解析；`alias::local-id` 先映射到导入 Scope；已经是规范 ID 的引用也可直接解析。
- 活跃 `project` Scope 可以导入一个或多个 `environment` Scope。相对 import path 以活跃 Registry 根目录为基准；alias 和 Scope ID 都必须唯一。
- 当前 import 是一层对象加载：读取导入 Scope 的 manifest 以及其 `entities/links/routes`，不继续处理导入 manifest 的 imports 或 bindings。活跃 Scope 为 `environment` 时不允许 imports。
- Binding 只来自活跃 manifest，格式为 `role: entity-ref`；加载时解析并保存为规范 Entity ID。`ResolveEntity` 和 `show` 优先把输入当 Binding，再按 Entity 引用解析。
- Link 的 `from/to` 与 Route step 的 Link 引用在加载校验时改写为规范 ID。
- 加载校验确认引用存在、Link 有 Provider、Route 非空，并按 step 顺序检查每个 Link 的 `requires` 是否已由更早 step 的 `provides` 提供。当前不校验相邻 step 的 `to/from` 连通性。
- `documentation` 当前只是 Entity、Link、Route 输出中的静态 `{ref,title}` 元数据；没有扫描 `docs/`、解析文档内容或按上下文发现文档的实现。

## 3. 当前 Resolve

### 3.1 输入与适用性

CLI 输入为：

- `<target>`：Entity local ref、规范 ID 或 Binding；
- `--capability <name>`：与 Link `provides` 做精确字符串匹配；
- `--from <entity>`：解析为 `RuntimeContext.CurrentEntity`，Resolve 必填；
- `--vantage <name>`：未提供时默认为 `host:<hostname>`；
- Registry、Provider registry 和 SQLite Store：由 CLI 内部装配。

一条显式 Route 只有同时满足以下条件才是候选：

1. Route 至少有一个 step，最后一个 Link 的 `to` 等于规范 target；
2. **每一个** step Link 的 `from` 都等于当前 `--from` Entity；
3. 每个 Link 的 Provider 已注册，Provider 字段校验和 NativeHint 渲染成功；
4. 所有 step 的 `provides` 并集包含请求 capability。

这是当前实现的适用性规则，不是自动寻路：Resolve 不生成 Route、不扩展图、不检查 PATH 中是否真的存在 Provider 可执行文件，也不因 Observation 成功或失败而过滤、排序候选。

### 3.2 基数、证据与输出

- 0 个候选：`unresolved`；
- 1 个候选：`resolved`，放入 `route`；
- 多个候选：`ambiguous`，按 Route 规范 ID 排序后放入 `candidates`，不做自动 ranking。

每个候选输出规范 Route ID、由末 Link 得到的 target、排序后的 `derived_provides`、聚合证据，以及每个 step 的 Link ID、Provider、NativeHint、Link evidence。Resolve 只调用 `Provider.Validate/Render` 并读取 SQLite，绝不调用 `Provider.Probe`。

Link evidence 取当前 `(subject=link-id, vantage)` 最新 Observation：不存在为 `unknown`；非零 `expires_at` 已过期为 `stale`；没有显式期限或尚未过期时沿用已存的 `success/failure`。Route 聚合优先级为：任一 failure → `failure`；全 success → `success`；其余若都有 Observation 且至少一个 stale → `stale`；否则 `unknown`。

CLI 对 `unresolved` 和 `ambiguous` 仍输出结构化结果，但退出码为 3。

## 4. 当前 Provider 清单与 safe probe

Provider registry 仅注册以下三项：

| Provider | Resolve 时要求的 `provider_data` | NativeHint | 当前 safe probe |
|---|---|---|---|
| `frp-stcp` | `config`、`local_host`、`local_port` | `frpc -c <config>` | 先执行 `frpc verify -c <config>`；成功后 TCP connect `local_host:local_port` |
| `ssh` | `user`、`host`、`port` | `ssh -p <port> <user>@<host>`；非空 `credential_ref` 原样列入 `credential_refs` | 先 TCP connect `host:port`；成功后执行 `ssh -G -p <port> <user>@<host>`，只展开客户端配置 |
| `salt` | `minion_id` | `salt <minion_id> test.ping --out=json` | 执行同一条 `salt ... test.ping --out=json` |

补充事实：

- Registry 装载会确认 Provider 已注册，并调用对应 `Validate` 检查必填字段、字符串类型、非空值和 1–65535 端口范围；各 Probe 在 dial 或启动进程前再次执行相同校验。
- TCP connect 自带 3 秒超时并受上层 context 限制。`probe --timeout` 默认 10 秒，Route 的全部 step 共用同一个 timeout；Provider 外部命令都通过该 context 取消。
- 外部命令的 stdout/stderr 不进入错误或 Observation；失败只保存操作名和 timeout、canceled、exit code 或 start failure 等稳定类别。
- `context.runtime.available_tools` 只通过 PATH 查找 `frpc`、`ssh`、`salt` 后排序展示，不是 Resolve 的可用性门禁。
- Probe 一个 Route 时按 step 顺序执行并逐条写 Observation，遇到首个 failure 即停止；失败结果退出码为 4。
- `credential_ref` 只是 NativeHint 中的 opaque reference，当前不会读取或注入 Secret。
- SSH Probe 成功只证明 TCP endpoint 可连且 `ssh -G` 能展开配置，不证明认证、建立 session 或远端 Shell；FRP Probe 不启动 tunnel；Salt Probe 只执行 `test.ping`。

## 5. Observation 与 SQLite

Observation 字段为 `subject`、`vantage`、`status`、`observed_at`、`expires_at`、`provider`、JSON `evidence` 和 `error`，SQLite 另分配自增 ID。

- Store 是 append-only `observations` 表；索引覆盖 `(subject, vantage, observed_at DESC)`。新 Probe 不覆盖历史行。
- 默认路径：Windows 为 `%LOCALAPPDATA%/locus-link/state.db`；其他平台优先 `$XDG_STATE_HOME/locus-link/state.db`，否则 `~/.local/state/locus-link/state.db`。`LOCUS_STATE_PATH` 可显式覆盖。
- Probe 开始时使用 UTC 时间并设 15 分钟有效期；FRP、SSH、Salt 分别写入 `frpc-config-and-tcp-connect`、`tcp-connect-and-ssh-config`、`salt-test-ping` evidence kind。声明错误在执行和 Append 前被拒绝；实际测量成功或失败才持久化。
- `Latest` 按 subject 与 vantage 隔离，依 `observed_at DESC, id DESC` 取一条；stale 是读取时根据过期时间派生，不回写数据库。
- `status <link>` 返回该 vantage 的最新 Link evidence；`status <route>` 聚合各 step 最新 evidence；无参数 `status` 汇总 Registry 内全部 Link 在该 vantage 下的 success/failure/stale/unknown 数量。

```mermaid
stateDiagram-v2
    [*] --> unknown: 尚无该 subject + vantage 的 Observation
    unknown --> success: safe probe 通过并 Append
    unknown --> failure: safe probe 失败并 Append
    success --> stale: 当前时间超过 expires_at
    failure --> stale: 当前时间超过 expires_at
    stale --> success: 后续 probe 成功并成为 Latest
    stale --> failure: 后续 probe 失败并成为 Latest
    failure --> success: 后续 probe 成功并成为 Latest
    success --> failure: 后续 probe 失败并成为 Latest
```

## 6. 当前 CLI 清单

除 help 外，根命令固定注册九个子命令；结果型命令输出 JSON，`web` 启动长运行的本机 HTTP 服务。

| 命令 | 当前行为 |
|---|---|
| `init` | 创建 `scope.yaml` 与 `entities/links/routes/docs` 目录 |
| `validate` | 加载并校验 Registry，返回活跃 Scope 与三类对象数量 |
| `context` | 返回 Scope、imports、bindings、RuntimeContext、Observation Store 路径 |
| `list [entity\|link\|route]` | 返回排序后的规范 ID |
| `show <ref-or-id>` | 展开 Binding 或返回 Entity/Link/Route 声明，不附加运行时证据 |
| `resolve <target> --capability <name>` | 按当前规则筛选显式 Route 并附加既有证据 |
| `probe <link-or-route-id>` | 执行 safe probe 并追加 Observation |
| `status [link-or-route-id]` | 查看最新 Link/Route evidence 或按状态汇总 |
| `web` | 装载 Registry context，启动 loopback HTTP server 并提供嵌入式 Vue Graph/Status/Knowledge 导航骨架 |

`context`、`resolve`、`probe` 需要 `--from`；`resolve` 还需要 `--capability`。`web` 的 `--from/--vantage` 是初始页面上下文；`web` 只允许 loopback `--address`。各命令可用 `--registry` 覆盖发现结果。

`validate`、`list`、`show` 只装载声明，不解析 runtime、PATH、vantage 或 Observation state；`context`、`resolve`、`probe`、`status` 分别按实际消费装配运行时或状态依赖。

## 7. 当前 E2E 基线

唯一 workspace E2E 是 `TestWorkspaceEndToEnd`，由 `test/reproduce.ps1` 运行；测试重建 `temp/e2e-run/`，结束后保留二进制、具现化 Registry、SQLite 和 probe log。

当前 case 只包含：

- 两个具现化 project（alpha、beta），各自导入同一个 `environment.customer-a`；
- workstation、host、FRP server 三类 Entity；
- `frp-stcp + ssh` 两步 Shell Route，以及单步 Salt Route；
- 工作区内的模拟 `frpc/ssh/salt` 可执行文件和真实本地 TCP listener。

E2E 已覆盖 `init`、严格命令参数、Registry 向上发现、validate、context、list、show；证明 Resolve 不触发 Probe；证明 Shell Route 的 FRP/SSH 顺序 Probe、直接 Link Probe、Salt success/failure/recovery、SQLite 追加计数、status 与 Resolve evidence 更新、不同 vantage 隔离，以及 unresolved/ambiguous 与“不按 evidence 排名”。

## 8. 公共契约符合度

当前公共契约与代码的已知偏差：

| 公共契约 | 当前实现 | 影响 |
|---|---|---|
| Resolve 返回 Binding 解释、target Entity facts 和 documentation refs | 当前只返回 input target、canonical target、Route、NativeHint 和 evidence | 公共 Resolve result 尚未完整实现 |

这些偏差是实现任务，不改变公共契约。修复后必须更新本表和 E2E 基线。

## 9. 明确未实现

以下能力在当前 Go model、CLI wiring、Provider registry 和 E2E case 中均无实现：

- PostgreSQL Provider、声明与 E2E case；Gitea CI/CD 的声明、Route 与 E2E case；
- 扫描 `docs/`、documentation discovery、文档内容加载或 progressive disclosure；
- Plan、Instance、Execute 阶段或通用执行器；NativeHint 只是 Provider-native 命令提示；
- 自动 Route discovery、路径搜索、候选 ranking；
- Web Graph/Status/Knowledge 的完整读取投影、Markdown 内容加载、Resolve 与 Probe 交互；

因此，设计文档中的任何上述目标都不能视为当前实现能力；新增实现后应先更新本快照中的清单和 E2E 基线。
