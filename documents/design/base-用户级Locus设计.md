# locus-link 用户级 Locus 设计

## 简述

用户级 Locus 是当前用户使用 locus-link 的本机入口。它由一个可独立校验的用户根 Scope 和一组本机状态组成：项目登记、Registry Source metadata、remote cache、refresh 状态与 Observation Store。

用户级 Locus 不复制项目或 remote Registry，也不把所有已登记项目自动合并为全局 Scope。项目内运行时以项目 Scope 为根，并通过项目 Registry 中的显式 import 接入用户 Scope；项目外运行时以用户 Scope 为根。

现行实现只有 embedded Registry discovery 和本机 Observation SQLite。本文定义目标模型；目录布局、SQLite schema 和公共命令由实现与公共契约另行固定。

## 职责

本文负责：

- 定义用户根 Scope 与本机状态的关系；
- 定义项目 Registry 的正向引用与用户登记表的反向登记；
- 定义项目内、项目外的根 Scope 选择；
- 定义 Git/URL remote cache、显式 refresh、原子激活和失败回退；
- 定义本机 Observation、credential reference 和 diagnostics 的归属。

本文不负责：

- 定义 Scope、Registry、Source、Import、alias 或回环收集算法；
- 固定用户目录、数据库 schema、缓存布局、Git/HTTP 命令或 wire format；
- 把 Profile、actor、MCP runtime 或通用 credential system 引入当前模型；
- 为 remote Scope 配置独立数据库或 Observation Store。

Scope graph 的完整语义由[基础 Scope 设计](base-Scope设计.md)定义。

## 组成

```text
用户级 Locus
├── 用户根 Scope/Registry
└── 本机状态
    ├── project registrations
    ├── Source/cache metadata
    ├── remote cache
    ├── refresh diagnostics
    └── Observation Store
```

用户根 Scope 使用与项目和 remote Scope 相同的 Registry 结构，可以显式 import directory、Git 或 URL Scope。它本身不因位于用户级 Locus 而获得特殊声明语义。

本机状态只保存引用、缓存和观测，不成为第二份 declaration truth source。

## 项目与用户的双向记录

### 项目到用户

项目 Registry 显式 import 当前用户 Scope。该边参与项目 Declared View：

```text
project Scope
└── explicit import → user Scope
    └── explicit imports → environment/shared/remote Scopes
```

项目因此通常具有最大的可见能力 closure，但用户和 remote declarations 仍由原 Scope 拥有。用户 Scope 不得通过默认值或隐式 attachment 注入项目。

项目中如何编码用户 Scope locator 由声明契约固定；无论采用简写还是完整形式，该引用都必须显式、可诊断，并在加载后验证用户 Scope 的 `scope_id`。

### 用户到项目

用户级登记表保存反向引用：

```text
project registration
├── project scope_id
├── project Registry Source/path
└── registration metadata
```

反向登记用于项目发现、迁移和管理，不是 import：

- 不复制项目 Registry；
- 不把项目声明并入用户 Scope；
- 不让一个项目自动看到另一个项目；
- 项目移动或删除后可以更新或移除登记，而不改变 Scope identity。

### 初始化与登记

`init` 与 registration 是两个可独立完成的职责：

1. 创建可独立校验的用户或项目 Registry；
2. 写入 Scope manifest、必要目录和显式 imports；
3. 完整校验 Registry；
4. 可选地把项目引用登记到当前用户级 Locus。

CLI 可以连续执行这些步骤，但 Registry 创建失败不得留下有效 registration，registration 失败也不得破坏已经创建并可独立使用的 Registry。

## 根 Scope 选择

默认入口只有两种：

```text
不在项目中运行 → 用户 Scope 为根
在项目中运行   → 项目 Scope 为根
```

项目对用户 Scope 的接入来自项目 Registry 的显式 import，而不是运行时隐式 merge。显式 `--scope`、`--registry` 或等价选择存在时，直接使用调用方指定的根并校验。

项目发现可以使用当前目录向上查找 embedded Registry，也可以使用用户登记表中的项目路径关联。多个发现结果不能按 first match 静默覆盖；相同根可去重，无法证明相同则报告冲突并要求显式选择。

## Locus 管理面

Web 的 Locus 页面是用户级稳定入口，内部由 Scope Catalog 与 Dependency Graph 两个页签组成：

- Catalog 读取用户根与项目反向登记，标记 available、missing、invalid 或 identity mismatch；
- 只有用户根和有效 registered Project Scope 可以作为独立 Root 打开；
- remote/cache Scope 可以在依赖图中选中、查看 provenance 和沿边导航，但不会隐式提升为 Root；
- 节点选中与打开 Scope 是两个操作；沿图检查不得隐式改变当前工作区；
- Dependency Snapshot 保留 Root digest、snapshot digest、collection time、complete/partial、Scope 节点、多 alias import edge 与 blocked diagnostics。

Locus 页面和 Scope 工作区是不同层次：前者负责发现、登记状态和依赖分析；后者负责具体 Root 下的 Graph、Status、Knowledge 与 Inspect。

## Remote cache 与 refresh

### 一般规则

Git 和 URL Source 必须先物化到当前用户级 Locus 的隔离缓存。普通读取、Resolve 和 Probe 不访问网络，只使用已激活版本；网络更新必须由显式 refresh 发起。

```text
refresh
→ fetch into isolated candidate
→ strict decode and validate Registry
→ recursively collect candidate imports through an overlay cache
→ build candidate Dependency Snapshot
→ diff against active Dependency Snapshot
→ auto-activate when no regression
→ otherwise retain active and require explicit confirmation
→ atomically activate all involved edge pointers and non-conflicting authorities
```

刷新失败或回退时：

- Candidate 自身无法严格装载、`scope_id` 不匹配或 containment 失败：禁止激活；
- 已有最后有效缓存：继续使用该版本，并记录 refresh error；
- 首次获取且没有有效缓存：阻断对应 import edge，其他 Scope 保持可用；
- 新增 blocked、cycle、authority conflict 或 completeness 回退：返回 active/candidate snapshot 与 diff，等待显式确认；
- 确认必须绑定已审阅的 candidate snapshot digest；重新获取后 snapshot 变化则重新审阅；
- 同一批 edge pointer 在单个 SQLite transaction 中切换，不允许逐边立即激活；
- 不得用 Observation 掩盖 Source 或声明错误。

缓存位置只是实现细节，不进入 Scope 或 object identity。

### Git

Git Source 可以请求 branch、tag 或 commit：

```text
requested_revision = main
resolved_revision  = immutable commit
content_digest     = digest of validated Registry content
```

普通命令固定使用已激活的 resolved commit。Refresh 才 fetch 并重新解析 mutable branch/tag；新 commit 只有在完整校验成功后才能原子激活。

### URL

同一 URL 的内容可能变化，因此 URL Source 以 validated content digest 标识实际使用内容。Refresh 可以利用 ETag 或 Last-Modified 执行条件请求，但二者只是下载优化，不能代替 content digest。

内容 digest 未变化时无需激活新版本；内容变化时必须先完整校验再替换。Source URI 不得包含 credential value。

### Source 修改

Source 切换不使用独立 authority-switch 协议：

```text
修改显式 import Source
→ 显式 refresh
→ 校验并激活新缓存
```

新 Source 的 Registry 必须具有预期 `scope_id`。切换成功不改变 Scope 或内部 object identity；失败继续使用最后有效内容，但 diagnostics 必须指出配置 Source 与当前 active cache provenance 的差异。

## Observation Store

Probe 在执行机器上产生 Observation，并只追加到当前用户级 Locus 的 Observation Store：

```text
validated Link
+ invocation context
+ Safe Probe
→ sanitized Observation
→ local Observation Store
```

- Declaration 始终留在 owning Registry，不复制到 Store；
- remote Scope 不接收 Observation，也不需要数据库；
- Route evidence 在读取时从 Link Observations 派生，不单独持久化；
- Observation 必须保留 declaration、Source content、vantage、effective binding 和 Probe semantics provenance；
- 项目登记和 cache metadata 可以与 Observation 使用同一数据库实现，但逻辑 ownership 必须分开。

## Credential 与本机配置边界

用户级 Locus 只允许保存 opaque credential reference。Git、HTTP、SSH 或其他 native 工具从其既有 credential mechanism 获取实际 Secret。

```text
允许：credential reference
禁止：password、token、private key、raw auth payload
```

Credential value 不进入 Registry、Source URI、cache metadata、Observation、diagnostics、错误、日志或公共结果。当前不建立通用 Credential Source domain model。

命令调用可以显式提供 current entity、vantage 和 workstation-local mechanism binding。命名 Profile 与独立 actor 语义保持冻结：它们不参与 Scope 收集，也不作为当前用户级 Locus 的必要模型。

## Diagnostics

用户级 diagnostics 至少能够回答：

- 当前根是用户 Scope、项目 Scope 还是显式选择；
- 项目 Scope 是否显式引用用户 Scope；
- 用户登记表是否存在对应项目反向记录；
- 每个 remote Source 当前使用哪个 resolved commit/content digest；
- 最近一次 refresh 是否成功，失败时继续使用哪个最后有效版本；
- 哪些 import edges 被阻断，以及 Declared View 是否 partial；
- Observation Store 位于哪个本机状态边界。

Documentation diagnostics 不是分析正文是否正确，而是证明文档属于哪个 Scope、关联哪个 declaration、来自哪个 Source revision/digest，并确认路径没有越出该 Scope 的 `docs/` containment。

## 验证不变量

- 用户根 Scope 与项目/remote Scope 使用同一种 Registry 模型；
- 项目到用户是显式 import，用户到项目是非 import 的反向登记；
- 登记项目不会把其声明自动并入用户 Scope；
- 项目内以项目 Scope 为根，项目外以用户 Scope 为根；
- 普通命令不隐式访问网络，只有 refresh 更新 remote cache；
- refresh 只有完整校验成功后才原子激活，失败保留最后有效版本；
- Git 实际运行内容可追溯到 commit，URL 内容可追溯到 digest；
- Source 或缓存迁移不改变 canonical identity；
- Probe 结果只写当前用户级 Observation Store，remote Scope 不携带状态库；
- Secret value 不进入 locus-link 的声明、缓存、状态或公共输出；
- 测试中的用户级 Locus、Registry、helper、cache 和 State DB 全部重定向到仓库工作区。
