# Locus Link Control Plane

## Control Plane Role

本控制平面管理 Locus Link 从窄 v0 到可执行 operational path 的连续演进。当前行动面仍是 declaration、Resolve、safe Probe / Observation、provider-native context 与 documentation references；长期允许在同一 Entity / Link / Route 模型上增加 Plan、Instance、Execute、Supervise、Teardown 和 WebUI。

迭代必须由真实 Project/Environment 和可运行 Vertical Slice 驱动。Registry 是当前存储/组织形态，不是产品本体；后续层不得复制 Locus Core、隐藏 provider-native semantics，或为尚未验证的能力预建空框架。

## Meta Reference

- 入口：非小规模 Locus Link 设计、CLI、Schema、Provider、Observation、documentation、WebUI 读取契约或 E2E 改动。
- 读取顺序：`cp-meta.md` → `cpm-locus-link.md` → 本文件 → `documents/design/README.md` → 相关代码与设计。
- 执行同时遵循根目录 `AGENTS.md`；代码和可运行测试是实现现状事实源，`documents/design/` 是确定设计入口。
- 创建或扩展 CP 使用 `create-control-plane`；阶段收尾或动态区膨胀使用 `compact-control-plane`。

## Scope

- 产品入口：`cmd/locus/main.go`、`internal/locus/`。
- 公共契约：`documents/design/contracts/`，维护用户、Agent 和 Registry 作者依赖的 CLI、JSON 与 YAML。
- 内部设计：`documents/design/系统设计.md`、`documents/design/数据流与存储设计.md`；当前 CLI 用户摘要：根目录 `README.md`。
- 实现快照：`documents/current-architecture.md`，记录当前代码与 E2E 已证明的事实及契约偏差。
- 背景资料：`documents/reference/Necessary Project Context.md`，提供问题来源和外部约束，不作为公共契约或当前实现事实源。
- E2E 声明：`test/e2e/case/`；测试驱动：`test/e2e_test.go`；复现入口：`test/reproduce.ps1`。
- 运行产物：`temp/e2e-run/`，必须保留供人工复现。
- 当前 v0 不实现 Planner、自动 path discovery、通用 Executor、Route Instance lifecycle、WebUI、Graph DB、远程 Registry、复杂 Store 或治理。
- 下一阶段允许规划 WebUI、Plan / Instance 语义验证和逐步 executable Route；未被真实案例需要前不创建 WebServer、Planner、Executor、Runtime、InstanceManager 或 GraphEngine 脚手架。

## Core Rules

### 安全与验证

- 所有 agent 读写、依赖缓存、helper、SQLite 和测试状态必须留在工作区内。
- 测试不得依赖本机安装的 FRP、SSH、Salt、PostgreSQL、Gitea 服务或用户凭据；外部行为使用工作区内确定性 helper。
- 可审阅的 E2E 声明和模拟设备状态放在 `test/e2e/case/`；物化副本放在 `temp/e2e-run/`。
- 新运行可以确定性替换 `temp/e2e-run/`，测试结束不得清理它。
- 行为改动以 `go test ./...` 及 `test/reproduce.ps1` 的完整闭环通过为完成条件。

### 产品与模型

- 人类核心模型是 Entity（东西）、Link（一步已知方式）、Route（已知 operational path）；Scope、Binding、Context、Observation 和 documentation refs 是支撑机制。
- v0 范围是 Declaration、Resolve、safe Probe / Observation、provider-native context 与 documentation references。
- Route 显式声明且不要求 `previous.to == next.from`；v0 无 automatic ranking、path discovery 或复杂 capability type system。
- `requires/provides` 保持轻量 capability flow：当前用于显式 Route 校验，未来可供 compilation / planning 消费。
- Declaration 保存已知访问方式；Instance 保存本次具体访问；Observation 保存实际测量。v0 不因未来层增加 runtime 抽象。
- Entity 保存目标稳定事实；Link / Provider config 保存机制用法；Binding 保存 Project role 映射；Runtime Context / Instance 保存调用时值。
- Secret value 始终由外部机制管理；Locus 只保存 reference，任何输出、错误和 Observation 都不得泄漏值。
- Provider safe probe passed 不等于完整 capability 已被证明可执行。
- Documentation reference 属于声明关系；内容不能覆盖结构化声明，也不参与 capability、Route 或 execution 语义。
- Registry 是实现/存储形态，不是最终产品边界。

### 演进约束

- 当前 Locus v0 不拥有通用执行器；长期允许 Route 被编译、实例化并执行，但必须调用 provider-native mechanism，禁止万能 `host.exec()`。
- CLI、Agent 与未来 WebUI 共享同一 Locus Core / truth source；WebUI 不复制 declaration、observation 或 documentation 模型。
- WebUI 下一阶段优先 read-oriented Graph、Status、Knowledge 三个视图；当前只稳定读取 contract，不提前建设复杂 API/framework。
- PostgreSQL 和真实 Gitea + FRP + worker CI/CD 是 first-class special cases，不再放入 Awaiting Real Need。
- 新字段、Provider 或抽象必须由上述真实案例证明；优先最小 schema/output 增量，不创建空接口、目录或兼容层。

### 变更同步

- CLI、JSON 或 YAML 变化时同步 `documents/design/contracts/`、README、E2E fixture 和断言；Core 或 Store 语义变化时同步系统设计与数据流设计。
- 长期可复用决策和坑点沉淀到 `cpm-locus-link.md`；一次性过程不进入 CPM。
- 设计更新不得把尚未实现的行为写成当前代码事实；实现状态由代码与 E2E 证明。

## Task Board

### Current Vertical Slice — WebUI

- [ ] 建立本机 HTTP 服务、Vue 工程和嵌入式静态资源，真实读取当前 Registry context。
- [ ] 完成 Graph、Status、Knowledge、Resolve、Validate 和 safe Probe 的 Web 体验；Profile/vantage 由启动默认值并允许页面切换。
- [ ] 完成 CLI/Web 共用 fixture 的浏览器集成测试，覆盖 evidence 更新、vantage 隔离、Markdown containment 和 Secret 边界。
- [ ] 每个阶段完成时运行 `go test ./...` 与 `test/reproduce.ps1`，保留 `temp/e2e-run/` 并提交阶段代码。

Workspace、Scope、Registry Source 和本地 Store 的目标边界以 [`documents/design/Workspace与Registry设计.md`](documents/design/Workspace与Registry设计.md) 为准；现行 `locus/v0` 公共契约在实现迁移前保持不变。

### Deferred Domain Cases

- [ ] PostgreSQL：冻结 database Entity stable facts，补 Provider、Resolve documentation discovery 和完整 E2E。
- [ ] Gitea + FRP + Worker CI/CD：验证显式 deploy Route 的最小 step input/output 语义。

代码证据和公共契约偏差统一维护在 [`documents/current-architecture.md`](documents/current-architecture.md)；Task Board 只保留可执行修改项。
