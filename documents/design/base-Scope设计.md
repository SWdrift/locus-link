# locus-link Scope 设计

## 简述

Scope 是 locus-link 收集和组合声明知识的基本节点。每个 Scope 具有稳定且唯一的 `scope_id`，其 Registry 保存该节点拥有的 Entity、Binding、Link、Route、显式 imports 和 documentation references；Registry Source 只说明从哪里取得这些内容。

项目、用户和远程环境使用同一种 Scope/Registry 结构。它们通过显式 import 组成可递归收集的图；调用方从一个根 Scope 出发，获得已成功加载的 transitive closure。Source、alias、缓存路径和导入链均不改变 Scope 或其内部对象的 canonical identity。

现行 `locus/v0` 仍只实现 embedded Registry、一层相对目录 import 和向上发现。本文定义目标模型；公共 YAML、CLI 或 JSON 契约的变化必须显式升级或迁移。

## 职责

本文负责：

- 定义 Scope、Registry、Registry Source、Import 和 Declared View；
- 定义 Scope 与内部声明的稳定 identity；
- 定义显式 import graph、alias、递归收集、去重和回环阻断；
- 定义目录、Git 和 URL Source 的共同语义；
- 定义部分可用、远程 revision/content digest 和来源 provenance。

本文不负责：

- 定义用户级项目登记、缓存目录、Observation Store 或本机 State DB；
- 固定公共 YAML schema、Git 命令、HTTP 协议或缓存文件布局；
- 定义 Entity、Binding、Link、Route、Resolve 或 Probe 的内部语义；
- 实例化或执行 documentation 中描述的 native/MCP 操作。

用户根 Scope、项目反向登记、remote cache 和本机状态由[用户级 Locus 设计](base-用户级Locus设计.md)定义。

## 一句话模型

```text
Scope    = 有稳定 identity 的声明节点
Registry = 一个 Scope 拥有的声明内容
Source   = Registry 的获取位置
Import   = 从一个 Scope 到另一个 Scope 的显式有向边
```

## Scope 与 Registry

### 统一结构

项目 Scope、用户 Scope 和 remote Scope 是使用角色，不是三套领域模型。所谓“环境 Scope”也只表示它相对于当前根 Scope 承载环境知识，不形成固定 `environment` 类型层级。

每个 Scope 的 Registry 使用同一种逻辑结构：

```text
Registry
├── Scope manifest
├── explicit imports
├── Entity declarations
├── Binding declarations
├── Link declarations
├── Route declarations
├── documentation references
└── docs/
```

Registry 必须能够脱离导入者独立解码和校验。Remote Scope 不携带 Catalog、缓存控制、项目登记、Observation Store 或本机数据库。

### Identity

所有 Scope 都必须在自身 Registry manifest 中声明稳定且唯一的 `scope_id`。本地目录、Git repository、URL、alias 和导入路径不能代替 `scope_id`。

```text
Scope identity  = scope_id
object identity = scope_id::local_id
```

同一 Scope 内，Entity、Binding、Link、Route 等声明使用各自契约规定的 local ID 规则；canonical identity 始终由 owning Scope 决定。Registry 从目录迁移到 Git 或 URL、缓存位置改变、或同一 Scope 经另一条 import path 到达时，identity 均保持不变。

Imported declaration 仍归原 Scope。Import 不复制声明，也不把 imported object 的 ownership 改成导入者。

## 显式 Import Graph

### 根与传导

每次收集从一个明确的根 Scope 开始：项目内通常是项目 Scope，项目外通常是用户 Scope。收集器只沿 Registry 中显式声明的 imports 继续加载；Catalog 中存在但未被 import 的 Scope 不进入本次 Declared View。

```text
Declared View
= root Scope
+ successfully collected transitive explicit imports
```

长传导链是正常结构：

```text
project
└── user
    └── customer
        └── platform
            └── shared
```

项目根通常具有最大的可见能力集合，但 imported declarations 仍由各自 Scope 拥有。

### Import 简写与完整形式

Import 可以使用简写，避免简单目录场景承担远程 Source 的配置复杂度；加载器必须先把所有形式规范化为同一内部语义。

示意简写：

```yaml
imports:
  customer: ../environment
  shared: https://example.test/locus/shared/
  platform: git+https://example.test/platform.git
```

需要预期 identity、revision 或 repository subpath 时使用完整形式：

```yaml
imports:
  customer:
    scope_id: environment.customer-a
    source:
      kind: git
      uri: https://example.test/environment.git
      revision: main
      path: registries/customer-a
```

以上只是设计示意，公共字段和版本由声明契约固定。规范化后的 Import 至少表达：

```text
local alias
optional expected scope_id
source kind and locator
optional requested revision
optional source subpath
```

每条 Import 都是显式边。Remote 完整形式宜声明预期 `scope_id`；取得 Registry 后，无论是否预先声明，加载器都必须读取其内生 `scope_id`，并校验预期值一致。

### Alias 与引用

Alias 是 importing Scope 给 import edge 分配的本地名称，用于形成可读的引用路径。Alias 不进入 canonical identity。

同一 Scope 可以经不同 alias 或不同路径到达：

```text
A1 → scope-a
A2 → scope-a
```

`A1::entity-x`、`A2::entity-x` 最终都解析为同一个 `scope-a::entity-x`，声明只加载一次。较长 alias path 也是正常现象，调用方始终可以使用 canonical identity。

Alias 只解决引用歧义：

- 同一 importing Scope 内，alias 必须无歧义；
- 相同 alias 指向不同 Scope 时，该 alias 不得绑定，diagnostics 提示修改 alias；
- 不带 Scope/alias 的引用存在多个候选时，调用方必须补充 alias path 或 canonical identity；
- alias 不能把同一 `scope_id` 的两份不同内容变成两个 Scope。

### 去重、回环与收集状态

收集器维护本次遍历状态，而不是把导入者信息写入下一级 Registry：

```text
visiting = 当前祖先 Scope 链
visited  = 已完整处理的 scope_id 集合
path     = 对应 import aliases 和 Sources
```

规则：

1. 目标 `scope_id` 已在 `visited`：复用节点并保留新的 alias path；
2. 目标 `scope_id` 已在 `visiting`：当前边是回边，阻断该边并报告完整回环路径；
3. 首次到达：加载、校验并递归收集其显式 imports；
4. 下一级 Scope 只声明自己的 imports，不需要知道谁导入了自己。

例如 `A → B → C → A` 只阻断 `C → A`；已成功加载的 A、B、C 保持可用。

### 相同 Scope 的 Source 冲突

多条路径取得相同 `scope_id` 时：

- immutable revision/content digest 相同：视为同一内容并去重；
- 内容不同且已有最后有效的 active cache：继续使用该版本，阻断冲突 Source；
- 内容不同且没有已建立的有效版本：不得按遍历顺序选择赢家，冲突 Scope 不进入可用图；
- diagnostics 必须列出各 Source、revision/digest 和到达路径。

这是 identity/authority 冲突，不能通过 alias 消除，也不允许 precedence merge 或 declaration override。修改显式 Source 后执行 refresh 即完成 Source 切换，不建立独立 authority-switch 协议。

## Registry Source

### 支持的来源语义

默认 Source 类型为：

| Source | 定位信息 | 不可变追溯信息 |
|---|---|---|
| relative directory | 相对 importing Registry 的目录 | validated content digest |
| absolute directory | 本机绝对目录 | validated content digest |
| Git | repository、requested revision、可选 subpath | resolved commit + content digest |
| URL | Registry 文件、目录描述或归档 URL | content digest |

Source locator、credential、缓存路径和 requested mutable revision 不进入 canonical identity。Git branch/tag 可以作为 requested revision，但实际使用内容必须记录 resolved commit；URL 内容以校验后的 digest 标识。

认证由 Git、HTTP 或其他外部机制处理。Registry 和公共 diagnostics 只允许 opaque credential reference，不保存 credential value，也不把 credential 注入 Source URI。

### Remote Registry 与缓存

Git 和 URL Registry 在进入收集图前必须经过：

```text
fetch into isolation
→ strict decode
→ validate scope_id and declarations
→ validate documentation containment
→ calculate immutable provenance
→ atomic cache activation
```

普通 Resolve、Probe 和读取操作只消费已激活缓存，不隐式访问网络。显式 refresh、最后有效版本和本机缓存的职责由[用户级 Locus 设计](base-用户级Locus设计.md#remote-cache-与-refresh)定义。

## 部分可用与 Diagnostics

单条 import 的回环、alias 冲突、首次远程获取失败或无 authority 的内容冲突，不应拖垮图中其他已校验节点。收集结果必须明确区分：

```text
completeness: complete | partial
blocked_imports:
  - source_scope
  - alias_path
  - source
  - reason
```

- `list`、`graph` 和 `show` 可以返回已加载内容，同时附加 blocked-import diagnostics；
- `probe` 可以测量已完整加载并校验的 Link；
- `resolve` 可以返回已发现候选，但 partial 结果不能声称候选集合完整；
- `validate` 必须报告被阻断的边和不完整状态；
- refresh 失败但仍使用最后有效缓存时，图可以保持 complete，同时报告 refresh error 和所用 revision/digest。

阻断必须是显式、稳定且可追溯的；不得静默选择 first match。

## Documentation 边界

Documentation reference 属于引用它的 declaration，并继承 owning Scope 和 Source provenance。正文必须限制在该 Scope Registry 的 `docs/` containment 内；remote 文档还应能追溯到 resolved commit 或 content digest。

文档内容对 Core 是按需读取的资料。文档可以描述 native 或 MCP 操作，但 locus-link 当前不解析、实例化或执行其中的 MCP 功能，也不建立独立 MCP Provider、Adapter 或 Resource 模型。

## 验证不变量

- 所有 Scope 都有内生 `scope_id`，所有内部对象 identity 均由 owning Scope 确定；
- 所有 import 都显式声明，简写和完整形式规范化后语义一致；
- 同一 Scope 经多条路径导入时只加载一次，但保留所有 alias/source provenance；
- 回环只阻断回边，不修改任何 Registry，也不使其他已校验节点失效；
- partial graph 不伪装成完整候选集合；
- alias 解决引用歧义，不掩盖相同 `scope_id` 的内容冲突；
- directory、Git 和 URL Source 使用同一种 Scope/Registry 模型；
- Source、revision、缓存或导入路径变化不改变 canonical identity；
- remote 内容使用 immutable resolved commit/content digest，并在激活前完整校验；
- remote Scope 不携带本机数据库或 Observation；
- Secret value 不进入 Registry、Source URI、缓存 metadata、diagnostics 或公共结果。
