# Locus Link Memory

本文件保存 `cp-locus-link.md` scope 内已验证、后续迭代可复用的结论。当前代码或高优先级规则与本文冲突时，以代码和高优先级规则为准。

## Product Decisions

- Locus Link 是带运行证据的 situated operational context，不是执行器。
- 长期核心为 Scope + additive import、Binding、canonical identity、窄语义 Link、显式 ordered Route、Declaration/Observation 分离、vantage 和 provider-native context。
- Project 与 Environment 是 declaration ownership/lifecycle Scope，不是 Graph Node。
- Binding 只表达 Project role → canonical Entity，不是 Link，也不提供 capability。
- v0 不建立 Local Capability 对象；Capability 是 Link `requires/provides` 字符串与 Resolve 查询条件。
- Route 的 target 由最后一个 Link 的 `to` 推导，provides 由 ordered steps 累计推导；不重复声明 target/provides/priority/generic constraints。
- Route 不要求严格节点连续。FRP→SSH 通过 `tcp-forward.ssh` 的 provides/requires 组合，不创建 localhost endpoint、session 或 tunnel Entity。
- Observation v0 只记录 canonical Link subject + vantage；Route 状态实时聚合，不落库。
- Observation Store v0 是本机 SQLite，不建设 shared/global/remote Store abstraction。
- CLI 外层 contract 是 `resolve / probe`；其余一级命令按 inspection 与 registry authoring 分层。

## Verified Implementation Baseline

- Go module，单一 `locus` executable；实现集中在 `internal/locus/`，没有 app/domain/ports 脚手架。
- Project 可以按本地路径 import Environment，alias 归一化到 `<scope-id>::<local-id>` canonical identity。
- 已实现 FRP、SSH、Salt Provider：Validate、Render NativeHint、safe Probe；无通用 Execute。
- FRP Probe 使用 `frpc verify` 和现有本地 endpoint；SSH Probe 使用 TCP 与 `ssh -G`；Salt Probe 只调用 `test.ping`。
- Resolve 不启动 FRP、不建立 SSH session、不执行 Salt、不调用 Probe；Probe 只验证既有状态并追加 Link Observation。
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

## Session Milestones

- `fdc4c61`：初始设计、Go CLI、FRP/SSH/Salt 前的基础实现与 E2E 基线。
- `a4ad924`：修复多 Project E2E 初始化 cwd。
- `4632fe1`：加入 Salt Provider、持久复现入口和保留产物规则。
- `ff18ca1`：将可审阅 E2E fixture 固化到 `test/e2e/case/`，增强完整 Context/关系/路径断言。
- `54e79b5`：增加面向使用者 README。
- `4de7d41`：补齐 Salt NativeHint、status、failure/recovery 与 Beta vantage 覆盖。
