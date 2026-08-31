# locus-link Control Plane

## Control Plane Role

本控制平面管理 locus-link 作为 concrete environment / situated operational knowledge layer 的演进。当前行动面是 declaration、Resolve、safe Probe / Observation、provider binding、documentation、CLI/WebUI，以及未来薄层 locus-link MCP Agent interface。

locus-link 控制面覆盖 concrete environment knowledge 与 native/MCP mechanism binding。

## Meta Reference

- 入口：非小规模 locus-link 设计、CLI、Schema、Provider、Observation、documentation、WebUI 读取契约或 E2E 改动。
- 读取顺序：`cp-meta.md` → `cpm-locus-link.md` → 本文件 → `documents/design/README.md` → 相关代码与设计。
- 执行同时遵循根目录 `AGENTS.md`；代码和可运行测试是实现现状事实源，`documents/design/` 是确定设计入口。
- 创建或扩展 CP 使用 `create-control-plane`；阶段收尾或动态区膨胀使用 `compact-control-plane`。

## Scope

- 产品入口：`cmd/locus/main.go`、`internal/locus/`。
- 公共契约：`documents/design/contracts/`，维护用户、Agent 和 Registry 作者依赖的 CLI、HTTP API、JSON 与 YAML。
- 基础设计：`documents/design/base-核心概念.md`、`documents/design/base-系统设计.md`、`documents/design/base-Home与Registry设计.md`、`documents/design/base-数据设计.md`；当前 CLI 用户摘要：根目录 `README.md`。
- 实现快照：`documents/current-architecture.md`，记录当前代码与 E2E 已证明的事实及契约偏差。
- 背景资料：`documents/reference/Necessary Project Context.md`，提供问题来源和外部约束，不作为公共契约或当前实现事实源。
- MCP 分层背景参考：[`documents/reference/locus-link 与 MCP.md`](documents/reference/locus-link%20与%20MCP.md)；确定设计仍以 `documents/design/` 为准。
- E2E 声明：`test/e2e/case/`；测试驱动：`test/e2e_test.go`；快速入口：`scripts/test-e2e.ps1`。
- 运行产物：`temp/e2e-run/`，必须保留供人工复现。
- 当前 v0 不实现 locus-link Home Catalog、managed/remote Source、Profile、MCP Adapter/MCP provider binding 或远程 Store。
- 当前 WebUI 是 loopback 本机界面：读取 Graph、Status、Knowledge，复用 Core 执行 Resolve 与显式 safe Probe；不引入远程服务。

## Core Rules

### 安全与验证

- 所有 agent 读写、依赖缓存、helper、SQLite 和测试状态必须留在工作区内。
- 测试不得依赖本机安装的 FRP、SSH、Salt、PostgreSQL、Gitea 服务或用户凭据；外部行为使用工作区内确定性 helper。
- 可审阅的 E2E 声明和模拟设备状态放在 `test/e2e/case/`；物化副本放在 `temp/e2e-run/`。
- 新运行可以确定性替换 `temp/e2e-run/`，测试结束不得清理它。
- 行为改动以 `scripts/verify.ps1` 的完整闭环通过为完成条件；聚焦 E2E 诊断时使用 `scripts/test-e2e.ps1`。

### 产品与模型

- 人类核心模型是 Entity、Binding、Link、Route；Scope 是 ownership/namespace/composition boundary，Context、Observation 和 documentation 是支撑机制。
- Registry 是一个 Scope 的声明集合；Registry Source 是物理来源；canonical identity 不随 Source 位置变化。
- Declared View 只由 active Scope 与显式 transitive import DAG 构成；Profile/Situated Context 不注入普通声明。
- 一个 Home 内一个 Scope 同时最多一个 active authoritative Source；Source 迁移是 authority switch，不是 precedence merge。
- Route 显式声明且不要求 `previous.to == next.from`；Entity relation 与 `provides/requires` capability relation 并存。
- Secret value 始终由外部 native/MCP credential mechanism 管理；locus-link 只保存 reference。
- Provider Registry 保存 Link 与 native/MCP mechanism 的 concrete binding。
- Observation applicability 目标包含 declaration digest、vantage、provider binding、Probe kind/version 与 relevant context fingerprint；Profile 不能整体作为 validity key。
- Documentation reference 属于声明关系；内容不能覆盖结构化声明，适合由 MCP Resource 渐进暴露。

### 演进约束

- Plan、Instance、Execute、Supervise、Teardown 设计冻结；只有数据库与 Gitea E2E 证明 Agent + native/MCP providers 仍不可靠时才重新打开最小语义。
- CLI、WebUI 与未来 locus-link MCP Adapter 是同一 Core 的并列 clients；Core 不依赖 MCP。
- PostgreSQL 和真实 Gitea + FRP + worker CI/CD 是边界验证案例；真正 action 由 native 工具或已有 MCP Server 完成。
- 新字段、Provider 或抽象必须由上述真实案例证明；不创建 placeholder schema、空接口或兼容层。

### 变更同步

- CLI、HTTP API、JSON 或 YAML 变化时同步 `documents/design/contracts/`、README、E2E fixture 和断言；Core 或 Store 语义变化时同步系统设计与数据流设计。
- 长期可复用决策和坑点沉淀到 `cpm-locus-link.md`；一次性过程不进入 CPM。
- 设计更新不得把尚未实现的行为写成当前代码事实；实现状态由代码与 E2E 证明。

## Task Board

### Next Minimal Implementation

- [ ] 为 Observation 增加 declaration digest、Probe semantics version 与 relevant context fingerprint，迁移旧记录为 diagnostics-only。
- [ ] 先定义 locus-link MCP 最小公共契约，再以薄 Adapter 映射 `resolve/probe/status` 与 context/entity/link/route/doc Resources；Core 不引入 MCP SDK 类型。
- [ ] 以 database fixture 验证 native 与 MCP provider binding 的最小声明差异。

### Deferred Domain Cases

- [ ] locus-link Home/Catalog、managed/remote Source、authority switch 与 immutable revision provenance。
- [ ] Gitea + FRP + Worker + Production：验证陌生 Agent 能获得结构化 Route/capability/provider/evidence/docs，而不是原样 YAML/README。

代码事实和目标设计偏差统一维护在 [`documents/current-architecture.md`](documents/current-architecture.md)；Task Board 只保留可执行修改项。
