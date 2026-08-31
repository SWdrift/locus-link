# locus-link Memory

本文件保存 `cp-locus-link.md` scope 内已验证、后续迭代可复用的结论。当前代码或高优先级规则与本文冲突时，以代码和高优先级规则为准。

## Product Decisions

- Plan、Instance、Execute、Supervise、Teardown 当前冻结；只有真实 E2E 证明现有 Resolve、显式 Probe 和 native provider 仍缺少必要语义时，才重新打开窄设计。
- 命名规则：项目/产品统一为 `locus-link`；`locus` 只表示 executable、schema/API/resource namespace；“用户级 Locus”专指用户根入口与本机状态边界。
- Registry 是一个 Scope 的同构声明载体；Scope 是稳定 identity 节点，Registry Source 只负责定位内容。
- 面向人的核心模型是 Entity（operational resource）、Link（一步已知方式）、Route（已知 operational path）；Scope、Binding、Context、Observation 和 documentation refs 是支撑机制。
- Project、User 与 Environment/remote 是 Scope 在当前收集图中的相对角色，不形成不同 Graph Node 或固定 Scope 类型。
- Declaration 保存已知访问方式；Situated Context 保存本次现场；Observation 保存特定 Probe 语义下的实际测量。三者不得互相覆盖。
- 参数所有权：Entity 保存目标稳定事实；Link/provider binding 保存机制用法；Binding 保存角色映射；Situated Context 保存调用时值。
- locus-link 只保存 Secret reference；Secret value 由 SSH Agent、Credential Manager、Vault、环境变量等外部机制负责。
- Provider 外部进程的 stdout/stderr 属于不受信输入，不进入错误、Observation 或日志；当前诊断只保留操作名和稳定失败类别，未来只能增加 Provider 明确解析并列入白名单的非敏感字段。
- v0 不建立 Capability 对象；`requires/provides` 是开放字符串与轻量 fold，用于校验显式 Route。
- Route 的 target 由最后一个 Link 的 `to` 推导，provides 由 ordered steps 累计推导；不重复声明 target/provides/priority/generic constraints。
- Route 不要求严格节点连续。FRP→SSH 等链路表达前一步为后一步建立条件，不创建 localhost endpoint、session 或 tunnel Entity。
- Observation provenance/适用性目标包含 canonical Link、declaration digest、vantage、provider binding、Probe kind/version 与 relevant context fingerprint；Profile 与 actor 当前冻结。Route 状态实时聚合，不落库。
- Documentation reference 属于 declaration graph，供 Agent progressive context disclosure；文档内容不覆盖结构化声明，也不参与 capability、Route 或 execution 语义。文档可以描述 MCP 操作，但 locus-link 不解析、实例化或执行其中的 MCP 功能。
- Documentation reference 必须是相对路径，词法归一化和符号链接解析后都受限于所属 Scope Registry 的 `docs/`。
- CLI 与 WebUI 是同一 Core 的 clients；MCP Adapter、MCP Provider、MCP Resource 与 MCP SafeProbe 当前冻结。
- Observation Store v0 是本机 SQLite，不建设 shared/global/remote Store abstraction。
- CLI 当前外层 contract 是 `resolve / probe`；其余一级命令按 inspection 与 registry authoring 分层。
- 用户级 Locus 由用户根 Scope 与本机状态组成；本机状态管理项目反向登记、remote cache/refresh metadata 和 Observation Store，不建立持久化 Workspace domain object。
- Scope 是稳定 identity 节点；Registry 是其声明集合；Registry Source 支持相对/绝对目录、Git 和 URL，canonical identity 与 Source location 分离。
- Declared View 从明确根 Scope 沿显式 imports 递归收集；按 `scope_id` 去重，阻断回边并显式报告 partial。项目对用户 Scope 的引用也是显式 import。
- Import alias 解决引用歧义，不改变 canonical identity，也不能掩盖相同 `scope_id` 的内容冲突。
- 普通命令只读已激活 remote cache；显式 refresh 获取并校验 Git/URL 内容，记录 resolved commit/content digest，成功后原子激活，失败保留最后有效版本。
- Credential value 由 Git、HTTP、SSH 等外部机制管理；locus-link 只保存 opaque reference。

## Scope v1 Decisions

- 公共声明 clean cutover 到 `api_version: locus/v1`；不兼容解析 `locus/v0`、`scope.kind`、list-style imports 或 `--scope-kind`，不保留 alias、deprecated path 或 shim。
- 每个 Registry 内生 `scope_id`，对象 canonical identity 为 `scope_id::local_id`；Binding 属于 owning Scope。Project、User 和 remote 仅由根选择与 Source provenance 表达相对角色。
- Import 只允许 map 形式。Scalar path 与 `${LOCUS_HOME}` 前缀规范化为 directory，`git+https/git+ssh/git+file` 为 Git，`http/https` 为 ZIP URL；完整形式只有 expected `scope_id` 与 Source。
- `${LOCUS_HOME}/registry` 是唯一允许展开的变量，仅用于项目显式导入当前用户 Registry；其他占位符或环境变量一律拒绝。
- URL payload 固定为 ZIP Registry。下载与展开均受大小、entry 数、path traversal、drive path、symlink 和 containment 限制；不尝试单文件索引协议。
- 图计算固定使用 `gonum.org/v1/gonum v0.16.0` 的 `graph/simple`、`graph/path`、`graph/topo`；业务层不手写可达性、最短路径、SCC 或第二套 adjacency。
- Directory Source 直接读取；Git/URL 普通收集只读 edge active cache，缺失时报告 `missing_active_cache`，绝不 fetch。
- Git refresh 固定解析 mutable requested revision 到 immutable commit；URL refresh 可以使用 ETag/Last-Modified，但 active identity 仍由 validated content digest 决定。
- Refresh 在隔离 candidate 中完成严格校验后，才通过单个 SQLite transaction 激活 edge entry 与 Scope authority；失败保留 last-known-good，首次失败只阻断对应 edge。
- Source 修改后以显式 refresh 直接切换 active authority，不实现独立 authority-switch。相同 `scope_id` 的不同内容无 active authority 时全部排除，不按遍历顺序选赢家。
- PostgreSQL、Gitea、Profile、actor、MCP Adapter/Provider/Resource/runtime 和执行层保持冻结，不属于 Scope v1 后端落地的当前验收。

## Documentation Architecture

- 用户、Agent、Registry 作者和自动化直接依赖的 CLI、HTTP API、JSON 与 YAML 统一放在 `documents/design/contracts/`，视为需要兼容或显式升级的公共契约。
- Provider Go interface、Resolver、Observation Store 和 SQLite schema 当前是内部契约或实现细节，不因被记录而成为外部兼容承诺。
- `documents/design/base-核心概念.md` 维护 Entity、Link/Graph、Binding、根 Scope Route overlay、Resolve、Probe、Observation 等名词与产品生命周期。
- `documents/design/base-Scope设计.md` 维护 Scope、Registry Source、显式 import graph、alias、去重、回环阻断和 remote provenance；`documents/design/base-用户级Locus设计.md` 维护用户根入口、项目反向登记、remote cache/refresh 和本机 Store 归属；`documents/design/base-系统设计.md` 从 Declared View 开始维护 Resolve / Probe → Provider → Observation 的 Core 处理链。
- `documents/design/base-数据设计.md` 维护来源、变换、ownership、持久化、freshness 和 Secret 血缘，不复制公共 schema 或 SQLite 具体表结构。
- `documents/current-architecture.md` 记录当前代码映射、支持范围和公共契约偏差，不承担目标契约。

## Forward Compatibility Boundaries

- PostgreSQL 的 `production-db` Binding、database Entity、explicit Route、credential/docs 与 Observation，以及 Gitea 的 source/worker/production Route 继续作为未来现实案例，不是当前 Scope v1 验收入口。
- 是否新增 Provider binding 仍取决于 Link 是否需要独立 Validate/Describe/SafeProbe contract，而不是对象名称。
- Resolve 与 Route Probe 只用第一条 Link 的 `from` 判断调用起点；后续步骤由显式 Route 顺序和 capability fold 约束，不要求每条 Link 都从 current Entity 出发。
- 只有既有 Scope/Resolve/Probe slice 的 E2E 明确证明缺失必要语义时，才重新打开 PostgreSQL、Gitea、MCP 或执行层设计。

## Verified Implementation Baseline

- Go module，单一 `locus` executable；Cobra adapter 位于 `internal/cli/`，领域模型、Resolver、Provider 和 Store 位于 `internal/locus/`。
- 本机 Web 入口位于 `internal/web/`，通过 `locus web` 启动 loopback HTTP server；Vue/Vite 资源编译后嵌入同一 executable。`/api/v0` 直接复用 Core 提供 Context、Graph、Status、Knowledge、Validate、Resolve 与 safe Probe。
- Graph 投影不暴露 Provider data；Knowledge 只读取声明引用且经符号链接解析后仍位于所属 Scope `docs/` 的文件，同路径多 association 去重；Markdown 禁用 HTML 并经 DOMPurify 净化。
- Web 只接受 loopback listener 与本机 Host，拒绝 cross-site fetch；读取和 Resolve 不写状态，只有显式 Probe 追加 Link Observation。
- Project 可以按本地路径 import Environment，alias 归一化到 `<scope-id>::<local-id>` canonical identity。
- 已实现 FRP、SSH、Salt Provider：Validate、Render NativeHint、safe Probe；无通用 Execute。
- FRP Probe 使用 `frpc verify` 和现有本地 endpoint；SSH Probe 使用 TCP 与 `ssh -G`；Salt Probe 只调用 `test.ping`。
- Resolve 不启动 FRP、不建立 SSH session、不执行 Salt、不调用 Probe；Probe 只验证既有状态并追加 Link Observation。
- Registry 装载会校验对象版本、identity 字符集、Environment 限制、documentation containment 和 Provider 注册；非空声明 `provider_data` 必须完整，空值可由 runtime mechanism binding 提供，effective Link 在 Resolve/Probe/Status 时统一校验。
- Provider Probe 在 dial 或进程启动前重复执行 Validate；FRP、SSH、Salt Observation 分别记录具体测量 kind，外部进程失败不保存原始输出。
- Observation 的零 `expires_at` 表示没有显式期限；只有非零且已过期的记录才派生为 stale。
- Observation 持久化 declaration/source/binding digest、Probe kind/version、vantage 与 relevant context fingerprint；Resolve、Status 和 Route evidence 统一使用完整 applicability query，旧 schema 行保留但因 provenance 为空而失效。
- `validate`、`list`、`show` 是 declaration-only 路径，不解析 PATH、runtime 或 Observation state。
- `LOCUS_STATE_PATH` 可将测试 Store 定向到工作区；生产默认使用 OS 本机 state 目录。
- `context`、`resolve`、`probe`、`status` 统一要求显式 `--from`，并通过同一个 builder 解析 vantage 与 workstation-local `--mechanism-bindings`；未传 vantage 时退化为 host-specific value。

## End-to-End Contract

可审阅案例位于 `test/e2e/case/`：

- 共享 `environment.customer-a`，包含 production host 与 FRP server。
- Project 模板物化为 `project.alpha` 和 `project.beta`。
- Binding `production-host` 指向 `environment.customer-a::host.prod-01`。
- Alpha/Beta 使用不同 workstation canonical identity 和 vantage。
- FRP→SSH Route 与 single-Link Salt Route 共用同一 Scope/Binding/Observation 机制。
- 模拟设备状态为 `frp-up / ssh-up / salt-up`，helper executable 位于运行时 `temp/e2e-run/bin/`。
- 两套 workstation-local FRP/SSH mechanism binding fixture 使用同一 Registry，concrete executable 不同但 canonical knowledge identity 不变。

完整 E2E 已验证：

- Cobra CLI 的 core、inspection、authoring 分层和实际 subprocess contract。
- active Scope、imports、Binding、cwd、current Entity、available Provider tools、vantage、Store path。
- `show binding` 显式保留 input ref、ref type 和 canonical target；show 不返回 evidence。
- Resolve 的 unresolved/resolved/ambiguous cardinality，不按 evidence 或其他因素自动选择。
- FRP/SSH Probe 写入两条 success Observation，再次 Resolve 变为 success。
- Salt NativeHint 精确为 `salt customer-a-prod-01 test.ping --out=json`。
- Salt success → failure（退出码 4 且 stdout 保持 JSON）→ recovery 会依次改变 Route status 与 Resolve evidence。
- Project Beta 在 `device-b` 下不会复用 Alpha/`office-lan` 的不适用 evidence，Resolve 保持 unknown。
- 同一 Registry 的 binding A Probe success 不会被 binding B 复用；两边 Resolve 的 Entity、Binding、Route、capability 和 documentation 保持一致。
- Observation 回归覆盖 declaration、binding、Probe version、vantage 变化导致失效，以及无关 CWD 变化不导致失效。
- 同一 E2E 启动真实 `locus web` 子进程并复用 CLI fixture/Store，验证 Graph/Status/Knowledge/Resolve/Probe、failure/recovery、vantage 隔离、文档去重与 containment、Secret 边界及嵌入式 UI 入口。
- 实际浏览器已验证 Graph、Status、Knowledge 三视图、Resolve、Probe、净化后的 Markdown 与窄屏无横向溢出；自动套件不依赖工作区外浏览器。

运行：

```powershell
./scripts/test-e2e.ps1
```

运行后必须保留 `temp/e2e-run/`：其中包含两个物化 Project、Environment、设备状态、helper、`locus.exe` 和 SQLite，供人工复现。

## Operational Lessons

- Go cache 和 module cache 必须放在 `temp/.go-cache`、`temp/.go-mod-cache`、`temp/.go-path`。非隐藏 `temp/go-mod-cache` 会被 `go test ./...` 当作 module 子树扫描并失败。
- Go module cache 文件可能是只读；若确需删除，应先用对应 `GOMODCACHE` 执行 `go clean -modcache`。当前规则是保留测试产物和缓存，不主动清理。
- E2E 的 FRP/SSH TCP listener 只在测试进程期间存活。测试完成后手动 `probe route.prod-shell` 失败是正确结果；重跑 `scripts/test-e2e.ps1` 才能复现完整成功闭环。
- Salt helper 不依赖 listener，可在保留的 `temp/e2e-run/` 中手动切换 `salt-up/salt-down` 观察 failure/recovery。
- E2E source fixture 应保持可审阅并提交；动态端口、绝对 FRP config path、Project ID 和 workstation 只在物化时替换。
- 新 Provider 应自报 executable；`Providers.Available` 从 Provider registry 推导并排序，避免新增 Provider 时维护重复列表。
- Registry YAML 使用 `go.yaml.in/yaml/v4` 严格解析；对象文件先加载为单个 `yaml.Node`，再按 `type` 解码，避免重复文件读取。v4 rc.6 的 `WithSingleDocument` 对 `yaml.Node` 不会拒绝多文档流，必须通过 Loader 显式确认后续读取为 `io.EOF`。

## Session Milestones

- `fdc4c61`：初始设计、Go CLI、FRP/SSH/Salt 前的基础实现与 E2E 基线。
- `a4ad924`：修复多 Project E2E 初始化 cwd。
- `4632fe1`：加入 Salt Provider、持久复现入口和保留产物规则。
- `ff18ca1`：将可审阅 E2E fixture 固化到 `test/e2e/case/`，增强完整 Context/关系/路径断言。
- `54e79b5`：增加面向使用者 README。
- `4de7d41`：补齐 Salt NativeHint、status、failure/recovery 与 Beta vantage 覆盖。
- `d684c34`：分离 `internal/locus` Core 与 `internal/cli` Cobra 适配层，抽取共用 Registry/runtime 装配。
- `4ff18fd`：建立 loopback Web 服务、嵌入式 Vue/Vite 骨架与 Graph/Status/Knowledge 导航。
- `a4d23a8`：完成 Core 读取投影、本机 `/api/v0`、Graph/Status/Knowledge、Resolve/Probe 体验与 Web 公共契约。
