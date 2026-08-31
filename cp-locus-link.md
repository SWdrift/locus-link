# locus-link Control Plane

## Control Plane Role

本控制平面管理 locus-link 作为 concrete environment / situated operational knowledge layer 的后端演进。当前行动面是 `locus/v1` Scope/Registry 契约、递归显式 import graph、用户级 Locus、remote cache/refresh、partial diagnostics、CLI 投影与后端 E2E。

WebUI、Profile、actor、MCP runtime、Provider/Resource 抽象和执行层保持冻结；本轮不修改前端控制平面或 `internal/web/ui/**`。

## Meta Reference

- 入口：非小规模 locus-link 设计、CLI、Schema、Provider、Observation、documentation、WebUI 读取契约或 E2E 改动。
- 读取顺序：`cp-meta.md` → `cpm-locus-link.md` → 本文件 → `documents/design/README.md` → 相关代码与设计。
- 执行同时遵循根目录 `AGENTS.md`；代码和可运行测试是实现现状事实源，`documents/design/` 是确定设计入口。
- 创建或扩展 CP 使用 `create-control-plane`；阶段收尾或动态区膨胀使用 `compact-control-plane`。

## Scope

- 产品入口：`cmd/locus/main.go`、`internal/locus/`、`internal/cli/`。
- 公共契约：`documents/design/contracts/`，维护用户、Agent 和 Registry 作者依赖的 CLI、JSON 与 YAML。
- 基础设计：`documents/design/base-核心概念.md`、`documents/design/base-Scope设计.md`、`documents/design/base-用户级Locus设计.md`、`documents/design/base-系统设计.md`、`documents/design/base-数据设计.md`；当前 CLI 用户摘要：根目录 `README.md`。
- 实现快照：`documents/current-architecture.md`，记录当前代码与 E2E 已证明的事实及契约偏差。
- E2E 声明：`test/e2e/case/`；测试驱动：`test/e2e_test.go` 与独立 Scope E2E；快速入口：`scripts/test-e2e.ps1`。
- 运行产物：`temp/e2e-run/native/` 与 `temp/e2e-run/scope/`，必须保留供人工复现。
- 当前实现任务：`locus/v1` clean cutover、递归 Scope collector、用户根与项目登记、Git/URL active cache 和显式 refresh、统一 partial CLI 与后端 E2E。
- 冻结边界：`internal/web/ui/**`、`cp-web-ui.md`、`cpm-web-ui.md`、Profile、actor、MCP Adapter/Provider/Resource/runtime、PostgreSQL/Gitea 新验收语义和 Plan/Execute/Supervise/Teardown。

## Core Rules

### 安全与验证

- 所有 agent 读写、依赖缓存、helper、SQLite 和测试状态必须留在工作区内。
- 测试不得依赖本机安装的 Git/FRP/SSH/Salt/PostgreSQL/Gitea 服务、用户配置、网络状态或凭据；外部行为使用工作区内确定性 helper 或 loopback server。
- 可审阅的 E2E 声明和模拟设备状态放在 `test/e2e/case/`；物化副本放在 `temp/e2e-run/`，测试结束不得清理。
- 每个后端阶段先运行 `go test ./internal/locus ./internal/cli`；完整交付运行 `scripts/test-e2e.ps1` 与 `go test ./...`。Go cache、module cache 和 GOPATH 必须定向到 `temp/`。
- 修改 Markdown 后运行 `pnpm --dir .tools/markdown run check:links`。

### Scope 与 identity

- 所有 Scope 内生 `scope_id`；所有对象 canonical identity 固定为 `scope_id::local_id`，Source、alias、路径、revision 和 cache 不得改变 identity。
- 所有 imports 显式声明并按 alias 稳定处理；收集按 `scope_id` 去重。回边只阻断当前 edge，已加载节点继续可用。
- 相同 `scope_id`、相同 content digest 合并 provenance；相同 `scope_id`、不同内容不得 first-match、merge 或由 alias 掩盖。无 active authority 时冲突 Scope 不进入可用 view。
- Partial 结果必须携带统一 `completeness` 与 `blocked_imports`；blocked diagnostics 使用稳定 reason，且不得泄露 external stdout/stderr、HTTP body、Secret 或 URI userinfo。
- Binding 属于 owning Scope；同一 Scope 内 Binding、Entity、Link、Route 的 local ID 统一唯一。

### 图算法

- 图算法固定使用 `gonum.org/v1/gonum v0.16.0` 的 `graph/simple`、`graph/path` 和 `graph/topo`。
- 不得自行实现可达性、最短路径或 SCC，也不得引入第二套邻接算法。业务层只编排可能失败的 Source edge、provenance、生命周期与稳定 diagnostics。

### Source 与 refresh

- 普通 `graph/list/show/context/resolve/status/probe` 只读 directory 或已激活 remote cache，绝不联网或隐式 fetch。
- Git/URL refresh 必须隔离获取、严格校验 expected/actual `scope_id`、声明与 docs containment，记录 immutable resolved commit/content digest，再原子激活。
- Candidate 失败不得改 active 指针；已有有效 cache 时继续使用最后有效内容，首次失败则只阻断对应 import。
- `${LOCUS_HOME}/registry` 是项目显式用户 directory import 唯一允许的变量展开；任何其他占位符或环境变量都拒绝。

### 变更同步与冻结

- CLI、JSON 或 YAML 变化时同步 `documents/design/contracts/`、README、后端 fixture 和断言；实现事实同步到 `documents/current-architecture.md`。
- 长期可复用决策和坑点沉淀到 `cpm-locus-link.md`；一次性过程不进入 CPM，未实现行为不得写成代码事实。
- WebUI、Profile、actor、MCP runtime、执行层及 PostgreSQL/Gitea 新语义保持冻结；不得为冻结边界创建 placeholder schema、兼容层或空接口。
- 前端路径和 WebUI 控制平面无论状态如何均不编辑、不暂存；共享 E2E 路径仅在工作树干净时迁移。

## Task Board

### Scope v1 Backend

- [ ] 固定 `locus/v1` 声明、identity、Binding、Source 与 composed view 契约。
- [ ] 实现 Gonum Scope graph、递归 collector、authority 冲突与统一 reference resolver。
- [ ] 实现 `LOCUS_HOME` 用户根、默认根选择和项目反向登记。
- [ ] 实现 Git/URL immutable cache、显式 refresh、原子激活与失败回退。
- [ ] 统一 Graph、Context、Validate、List、Show、Resolve、Probe、Status 的 partial CLI 投影。
- [ ] 迁移后端 fixture，补齐单元与独立 Scope E2E，并同步契约和实现快照。
