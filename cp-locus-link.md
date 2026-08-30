# Locus Link Control Plane

## Control Plane Role

本控制平面管理 Locus Link v0 从设计约束到 CLI、Provider、Observation 与端到端案例的持续迭代。它只维护当前行动边界、高频规则和下一步队列；稳定设计见 `documents/`，已验证结论与历史坑点见 `cpm-locus-link.md`。

迭代必须由真实 CLI 行为或可运行 Vertical Slice 驱动。若删除一个概念后现有闭环仍成立，优先删除或后置，不用新抽象补偿。

## Meta Reference

- 入口：非小规模 Locus Link 实现、CLI、Schema、Provider、Observation 或 E2E 改动。
- 读取顺序：`cp-meta.md` → `cpm-locus-link.md` → 本文件 → 相关代码与设计文档。
- 执行同时遵循根目录 `AGENTS.md`；代码和可运行测试是现状事实源。
- 创建或扩展 CP 使用 `create-control-plane`；阶段收尾或动态区膨胀使用 `compact-control-plane`。

## Scope

- 产品入口：`cmd/locus/main.go`、`internal/locus/`。
- 用户契约：根目录 `README.md` 中按 core、inspection、authoring 分层的 CLI。
- 设计边界：`documents/design-v0.md`、`documents/v0.md`。
- E2E 声明：`test/e2e/case/`；测试驱动：`test/e2e_test.go`；复现入口：`test/reproduce.ps1`。
- 运行产物：`temp/e2e-run/`，必须保留供人工复现。
- 不在本平面内扩张 Planner、Executor、Graph DB、远程 Registry package manager、复杂 Store、治理、Web 或 Project Manager。

## Core Rules

### 安全与验证

- 所有 agent 读写、依赖缓存、helper、SQLite 和测试状态必须留在工作区内。
- 测试不得依赖本机安装的 FRP、SSH、Salt 服务或用户凭据；外部工具行为使用工作区内 helper。
- 可审阅的 E2E 声明和模拟设备状态放在 `test/e2e/case/`；物化副本放在 `temp/e2e-run/`。
- 新运行可以确定性替换 `temp/e2e-run/`，测试结束不得清理它。
- 行为改动以 `go test ./...` 及 `test/reproduce.ps1` 的完整闭环通过为完成条件。

### 稳定产品边界

- CLI 外层核心固定为 `resolve / probe`；`context / show / list / status` 是 inspection，`init / validate` 是 authoring。
- v0 声明对象只有 Scope、Entity、Link、Route；Capability 只作为 `requires/provides` 字符串和 Resolve 查询条件。
- Scope import 是 namespaced additive composition；Binding 是 Project role → canonical Entity。
- Route 是显式 ordered Link chain，不要求 `previous.to == next.from`；合法性由 operational applicability 与 `requires/provides` 决定。
- Observation 只针对 canonical Link subject，并保留 vantage；Route evidence 从 Link Observation 聚合。
- Resolve 返回 provider-native context；Locus 不执行 Route。
- Resolve 的 0/1/多 Route 结果固定为 unresolved/resolved/ambiguous；不得隐式 ranking 或选择。
- 新字段、Provider 或抽象必须由真实 CLI 案例证明需要；不为后置项创建空接口、目录或兼容层。

### 变更同步

- CLI、Schema 或领域语义变化时，同步 README、`documents/v0.md`、E2E fixture 和断言。
- 高层长期不变量变化才修改 `documents/design-v0.md`。
- 可复用决策和坑点沉淀到 `cpm-locus-link.md`；一次性过程不进入 CPM。

## Task Board

### Ready

- [ ] 为 FRP/SSH 增加 workspace-local failure → recovery E2E，与 Salt 证据闭环对齐。
- [ ] 在第一个用户提供的真实 Project/Environment 上运行 CLI，记录缺失 Context 与关系表达问题。
- [ ] 用真实使用结果评估 `--from`、`--vantage` 和 Provider NativeHint 的易用性；只修正被案例证伪的设计。

### Awaiting Real Need

- [ ] 选择 Salt 之后的下一个异构 Provider；Gitea、Manual、DSC 在出现真实路径前保持后置。
