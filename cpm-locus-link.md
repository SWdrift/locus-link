# locus-link Memory

本文件保存 `cp-locus-link.md` scope 内已验证、后续迭代可复用的结论。当前代码或高优先级规则与本文冲突时，以代码和高优先级规则为准。

## Product Decisions

- Plan、Instance、Execute、Supervise、Teardown 当前冻结；只有数据库与 Gitea E2E 证明 Agent + native/MCP providers 仍不可靠，并给出缺失的最小语义时才重新打开窄设计。
- 命名规则：项目/产品统一为 `locus-link`；`locus` 只表示 executable、schema/API/resource namespace；`Locus` 只指上级项目。
- Registry 是当前实现/存储/组织形态，不是 locus-link 的最终产品边界。
- 面向人的核心模型是 Entity（operational resource）、Link（一步已知方式）、Route（已知 operational path）；Scope、Binding、Context、Observation 和 documentation refs 是支撑机制。
- Project 与 Environment 是 declaration ownership/lifecycle Scope，不是 Graph Node。
- Declaration 保存已知访问方式；Situated Context 保存本次现场；Observation 保存特定 Probe 语义下的实际测量。三者不得互相覆盖。
- 参数所有权：Entity 保存目标稳定事实；Link/provider binding 保存机制用法；Binding 保存角色映射；Situated Context 保存调用时值。
- locus-link 只保存 Secret reference；Secret value 由 SSH Agent、Credential Manager、Vault、环境变量等外部机制负责。
- Provider 外部进程的 stdout/stderr 属于不受信输入，不进入错误、Observation 或日志；当前诊断只保留操作名和稳定失败类别，未来只能增加 Provider 明确解析并列入白名单的非敏感字段。
- v0 不建立 Capability 对象；`requires/provides` 是开放字符串与轻量 fold，用于校验显式 Route。
- Route 的 target 由最后一个 Link 的 `to` 推导，provides 由 ordered steps 累计推导；不重复声明 target/provides/priority/generic constraints。
- Route 不要求严格节点连续。FRP→SSH 等链路表达前一步为后一步建立条件，不创建 localhost endpoint、session 或 tunnel Entity。
- Observation provenance/适用性目标包含 canonical Link、declaration digest、vantage、provider binding、Probe kind/version 与 relevant context fingerprint；Profile 只可作为 provenance，不能整体作为 validity key。Route 状态实时聚合，不落库。
- Documentation reference 属于 declaration graph，供 Agent progressive context disclosure；文档内容不覆盖结构化声明，也不参与 capability、Route 或 execution 语义。
- Documentation reference 必须是相对路径，词法归一化和符号链接解析后都受限于所属 Scope Registry 的 `docs/`。
- CLI、WebUI 与薄层 locus-link MCP Adapter 是同一 Core 的并列 clients；Core 不依赖 MCP。
- Observation Store v0 是本机 SQLite，不建设 shared/global/remote Store abstraction。
- CLI 当前外层 contract 是 `resolve / probe`；其余一级命令按 inspection 与 registry authoring 分层。
- locus-link Home 管理 Catalog、Profile、远程缓存和 locus-link State DB；不建立持久化 Workspace domain object。
- Scope 是 ownership、namespace 和 composition boundary；Registry 是一个 Scope 的声明集合；Registry Source 是物理来源，canonical identity 与 Source location 分离。
- Declared View 只含 active Scope 与显式 transitive import DAG；Situated Context 含 Profile、actor、vantage、provider availability 和相关 runtime facts，Profile 不注入普通声明。
- 一个 Home 内一个 Scope 同时最多一个 active authoritative Source；Source 迁移是 authority switch，不是 precedence merge。remote activation 必须记录 requested/resolved revision 与 content digest。
- MCP 负责 transport；locus-link Registry 保存 Link 所需 MCP binding/reference。
- Provider Registry 是 mechanism adapter/binding 层，允许 native 与 MCP 两类 concrete binding。

## Documentation Architecture

- 用户、Agent、Registry 作者和自动化直接依赖的 CLI、HTTP API、JSON 与 YAML 统一放在 `documents/design/contracts/`，视为需要兼容或显式升级的公共契约。
- Provider Go interface、Resolver、Observation Store 和 SQLite schema 当前是内部契约或实现细节，不因被记录而成为外部兼容承诺。
- `documents/design/base-核心概念.md` 维护 Entity、Link/Graph、Binding、Project Route overlay、Resolve、Probe、Observation 等名词与产品生命周期。
- `documents/design/base-Home与Registry设计.md` 维护 locus-link Home、Scope、Registry Source、项目关联、组合和本地 Store 归属；`documents/design/base-系统设计.md` 从 Canonical Declared View 开始维护 Resolve / Probe → Provider → Observation 的 Core 处理链。
- `documents/design/base-数据设计.md` 维护来源、变换、ownership、持久化、freshness 和 Secret 血缘，不复制公共 schema 或 SQLite 具体表结构。
- `documents/current-architecture.md` 记录当前代码映射、支持范围和公共契约偏差，不承担目标契约。

## Forward Compatibility Cases

- PostgreSQL：`production-db` Binding + database Entity facts + explicit Route + credential/docs + Observation，应能选择 native `psql` 或 existing PostgreSQL MCP binding；locus-link Core 不执行 SQL。
- Gitea：`source/ci-worker/production` Bindings 与 Gitea→FRP/bastion→worker→deploy mechanism Route，应让陌生 Agent 获得结构化 capability/provider/evidence/provenance。
- 是否新增 Provider binding 取决于某个 Link 是否存在独立 Validate/Describe/SafeProbe contract，而不是对象名称；MCP binding 只保存 reference。
- 当前 `Link.from == current_entity` applicability 可能阻塞 worker/deploy 或 tunnel 语义；先由两个案例证明失败，再决定是否增加最小 runtime condition/output。
- 下一步优先修复 Observation provenance/semantics validity，再建立薄 locus-link MCP contract/Adapter。

## Verified Implementation Baseline

- Go module，单一 `locus` executable；Cobra adapter 位于 `internal/cli/`，领域模型、Resolver、Provider 和 Store 位于 `internal/locus/`。
- 本机 Web 入口位于 `internal/web/`，通过 `locus web` 启动 loopback HTTP server；Vue/Vite 资源编译后嵌入同一 executable。`/api/v0` 直接复用 Core 提供 Context、Graph、Status、Knowledge、Validate、Resolve 与 safe Probe。
- Graph 投影不暴露 Provider data；Knowledge 只读取声明引用且经符号链接解析后仍位于所属 Scope `docs/` 的文件，同路径多 association 去重；Markdown 禁用 HTML 并经 DOMPurify 净化。
- Web 只接受 loopback listener 与本机 Host，拒绝 cross-site fetch；读取和 Resolve 不写状态，只有显式 Probe 追加 Link Observation。
- Project 可以按本地路径 import Environment，alias 归一化到 `<scope-id>::<local-id>` canonical identity。
- 已实现 FRP、SSH、Salt Provider：Validate、Render NativeHint、safe Probe；无通用 Execute。
- FRP Probe 使用 `frpc verify` 和现有本地 endpoint；SSH Probe 使用 TCP 与 `ssh -G`；Salt Probe 只调用 `test.ping`。
- Resolve 不启动 FRP、不建立 SSH session、不执行 Salt、不调用 Probe；Probe 只验证既有状态并追加 Link Observation。
- Registry 装载会校验对象版本、identity 字符集、Environment 限制、documentation containment、Provider 注册及完整 Provider data；声明错误不进入 Probe 持久化。
- Provider Probe 在 dial 或进程启动前重复执行 Validate；FRP、SSH、Salt Observation 分别记录具体测量 kind，外部进程失败不保存原始输出。
- Observation 的零 `expires_at` 表示没有显式期限；只有非零且已过期的记录才派生为 stale。
- 当前 Observation 只持久化 subject + vantage 等 v0 字段，缺少 declaration digest、Probe semantics version 与 context fingerprint；这是已确认的目标不变量缺口。
- `validate`、`list`、`show` 是 declaration-only 路径，不解析 PATH、runtime 或 Observation state。
- `LOCUS_STATE_PATH` 可将测试 Store 定向到工作区；生产默认使用 OS 本机 state 目录。
- 当前需要 Runtime Context 的命令要求显式 `--from`；未传 `--vantage` 时退化为 host-specific vantage。该易用性需由真实使用再评估。

## End-to-End Contract

可审阅案例位于 `test/e2e/case/`：

- 共享 `environment.customer-a`，包含 production host 与 FRP server。
- Project 模板物化为 `project.alpha` 和 `project.beta`。
- Binding `production-host` 指向 `environment.customer-a::host.prod-01`。
- Alpha/Beta 使用不同 workstation canonical identity 和 vantage。
- FRP→SSH Route 与 single-Link Salt Route 共用同一 Scope/Binding/Observation 机制。
- 模拟设备状态为 `frp-up / ssh-up / salt-up`，helper executable 位于运行时 `temp/e2e-run/bin/`。

完整 E2E 已验证：

- Cobra CLI 的 core、inspection、authoring 分层和实际 subprocess contract。
- active Scope、imports、Binding、cwd、current Entity、available Provider tools、vantage、Store path。
- `show binding` 显式保留 input ref、ref type 和 canonical target；show 不返回 evidence。
- Resolve 的 unresolved/resolved/ambiguous cardinality，不按 evidence 或其他因素自动选择。
- FRP/SSH Probe 写入两条 success Observation，再次 Resolve 变为 success。
- Salt NativeHint 精确为 `salt customer-a-prod-01 test.ping --out=json`。
- Salt success → failure（退出码 4 且 stdout 保持 JSON）→ recovery 会依次改变 Route status 与 Resolve evidence。
- Project Beta 在 `device-b` 下不会复用 Alpha/`office-lan` 的不适用 evidence，Resolve 保持 unknown。
- 同一 E2E 启动真实 `locus web` 子进程并复用 CLI fixture/Store，验证 Graph/Status/Knowledge/Resolve/Probe、failure/recovery、vantage 隔离、文档去重与 containment、Secret 边界及嵌入式 UI 入口。
- 实际浏览器已验证 Graph、Status、Knowledge 三视图、Resolve、Probe、净化后的 Markdown 与窄屏无横向溢出；自动套件不依赖工作区外浏览器。

运行：

```powershell
./test/reproduce.ps1
```

运行后必须保留 `temp/e2e-run/`：其中包含两个物化 Project、Environment、设备状态、helper、`locus.exe` 和 SQLite，供人工复现。

## Operational Lessons

- Go cache 和 module cache 必须放在 `temp/.go-cache`、`temp/.go-mod-cache`、`temp/.go-path`。非隐藏 `temp/go-mod-cache` 会被 `go test ./...` 当作 module 子树扫描并失败。
- Go module cache 文件可能是只读；若确需删除，应先用对应 `GOMODCACHE` 执行 `go clean -modcache`。当前规则是保留测试产物和缓存，不主动清理。
- E2E 的 FRP/SSH TCP listener 只在测试进程期间存活。测试完成后手动 `probe route.prod-shell` 失败是正确结果；重跑 `test/reproduce.ps1` 才能复现完整成功闭环。
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
