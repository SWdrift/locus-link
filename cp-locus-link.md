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
- 权威设计：`documents/design/`；当前 CLI 用户摘要：根目录 `README.md`。
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

- CLI、Schema 或领域语义变化时，同步权威设计、README、E2E fixture 和断言。
- 长期可复用决策和坑点沉淀到 `cpm-locus-link.md`；一次性过程不进入 CPM。
- 设计更新不得把尚未实现的行为写成当前代码事实；实现状态由代码与 E2E 证明。

## Task Board

### Awaiting User Discussion

- [ ] 讨论并冻结 PostgreSQL 与 Gitea CI/CD 的最小 implementation slice。
- [ ] 依据真实案例决定 Entity stable facts、documentation discovery 和 Provider 的最小 schema/output 改动。
- [ ] 验证当前 Route applicability 是否阻塞 worker/deploy path，再决定是否增加最小 step condition/output 语义。
- [ ] 冻结供 CLI、Agent 与下一阶段 read-oriented WebUI 共用的读取 contract。

## 演进路线

各阶段沿同一条 operational path 语义逐步增加能力。勾选状态随项目进展更新：

- [x] **Declaration**：声明 Entity、Link、Route、Binding 和相关文档，保存“我们知道应当怎样访问”。
- [x] **Resolve**：结合 Project、调用现场和 Observation，解释显式 Route，并返回原生工具所需的上下文。
- [ ] **Plan / Compilation**：把 Route 转换为明确、可检查的执行计划，补齐步骤依赖和运行输入。
- [ ] **Instantiate**：为一次具体调用解析凭据引用、临时端口和其他运行参数。
- [ ] **Instance**：表示这一次已经实例化的 Route 及其临时资源和状态。
- [ ] **Execute**：按计划调用 Provider 对应的原生机制，不抽象成万能 `host.exec()`。
- [ ] **Supervise / Teardown**：跟踪运行状态，并在完成或失败后释放隧道、会话等临时资源。
- [x] **Observation**：当前记录 safe probe 的实际证据；未来继续承接执行过程和结果的观测。
- [ ] **WebUI**：通过与 CLI、Agent 相同的 Locus Core 展示 Graph、Status 和 Knowledge。

```text
             Locus Core
          /      |       \
        CLI     Agent     WebUI
```

这是一条连续演进路线，不为 Planner、Executor、Runtime 或 WebUI 复制领域模型。后续层继续使用现有 Entity、Link、Route、provider-native mechanism、Observation 和 documentation references。
