# Locus Link Provider 设计

## 简述

Provider 是 Locus Link 保留异构原生机制的技术边界。每个 Provider 只负责校验自身声明、渲染结构化 native context，并执行范围明确的 Safe Probe；Locus 不把 SSH、FRP、数据库客户端、Salt 或后续机制压成通用远程执行接口。

本文只定义跨 Provider 稳定的契约与安全边界。当前已实现的 Provider、executable、声明字段和 probe 实现见[当前实现快照](../current-architecture.md)；Link 与 Resolve 的调用关系见[声明与解析设计](声明与解析设计.md)，证据语义见[Observation 设计](Observation设计.md)。

## 职责

本文负责：

- 定义 `Validate`、native context rendering 与 Safe Probe 的共同契约；
- 约束 Provider 注册、Secret 处理和外部进程安全；
- 约束新增机制保持 provider-native，而不污染 Link、Route 或 Observation；
- 规定未来实例化与执行仍由 Provider 明确定义原生操作。

本文不负责：

- 盘点当前 Provider 或固定某一实现的字段和命令；
- 自动安装、启动或配置外部工具；
- 管理 credential value、选择 Route、聚合 evidence 或访问 Observation Store；
- 提供通用 remote shell、file、SQL、deploy、service 或 `Execute(link, args)` 抽象。

## 稳定契约

Provider 以稳定 name 注册，并对自身机制提供三类能力：

### Validate

`Validate` 只校验对应 Link 中的 provider-specific declaration data：

- 必填字段、类型、取值和字段间约束由该 Provider 负责；
- 未注册 Provider 是声明错误；
- 校验不连接 endpoint、不运行 executable、不读取 Observation；
- 通用 identity、引用和 Route capability fold 仍由[声明与解析设计](声明与解析设计.md)负责。

Provider name 与原生 executable 可以不同，但都必须稳定且可解释。注册信息是 Provider 能力的唯一来源，不另行维护重复 executable 清单。

### Native context rendering

渲染把已校验的 Link 与 Runtime Context 转成供人、Agent 或后续实例化层使用的结构化原生上下文：

```text
NativeContext
├─ provider
├─ executable
├─ args[]
└─ credential_refs[]
```

- `args` 是逐项参数，不是 shell command string；
- `credential_refs` 只包含 Secret reference，不展开 Secret value；
- 渲染不运行 executable、不检查网络、不建立 session、不写 Observation；
- 平台或工具差异保留在 Provider 输出中，不改变 Link / Route 领域语义；
- Locus 不替调用方决定 shell quoting，也不通过 shell 重新解释参数。

Native context 是原生机制的可解释输入，不是已建立的连接或执行成功证明。

### Safe Probe

Safe Probe 是 Provider 明确定义的、非破坏性测量。它只检查该机制能够安全证明的最小事实，并返回一条可记录为 Link Observation 的结果：

- 使用调用方 context 控制 timeout 与取消；
- 只执行白名单内的安全操作，不接受任意原生命令或参数；
- 区分 `success`、`failure` 与无法形成结论的 `unknown`；
- 返回 provider、probe kind、实际测得范围、时间信息和已脱敏诊断；
- 不直接持久化结果，由 Probe orchestration 统一追加 Observation；
- 不修改 Declaration、credential、外部配置或远端业务状态；
- 不实例化或执行 Route，不把临时资源提升为声明。

Safe Probe success 只证明对应检查通过。例如 endpoint reachability、配置解析或原生 ping 成功，都不能自动证明认证、权限、完整 capability 或后续执行一定成功。Provider failure 是正常测量结果；声明非法、进程无法启动和持久化失败等系统问题必须与测量 failure 区分。

## Probe 与 Observation

Provider 只产生已脱敏的测量结果，不知道 Store、Route 聚合或候选选择。Probe orchestration 决定调用顺序和写入；Observation 定义 vantage、freshness、append/query 和证据强度。

这条边界保证：

- Provider 不因历史 success / failure 改写声明或渲染；
- Resolve 只渲染 native context，不触发 Probe；
- 同一个 Provider 无需为 Link probe、Route probe 或 Resolve 建立不同语义；
- 新 Provider 不引入专属 Store 表、Route 字段或 cardinality 规则。

## Secret 与进程安全

- Declaration 只保存 credential reference 或外部配置 reference；
- Validate、native context、Observation、错误和日志都不得包含 Secret value；
- stdout / stderr 在进入诊断或 evidence 前必须按 Provider 语义清理；
- executable 必须以参数数组直接启动，不经过 shell；
- timeout 与 context cancellation 必须终止本次 Probe 及其子进程；
- 取消、启动失败、非零退出、测量 failure 和解析失败必须可区分；
- Provider 不隐式读取用户凭据来补全声明，也不把外部配置内容复制进结果。

测试所用外部行为、凭据和状态边界由[测试设计](测试设计.md)统一规定。

## Provider 准入

新增 Provider 必须由真实路径证明，并同时满足：

1. 现有 Provider 不能自然表达该原生机制；
2. 声明字段具有具体机制语义，而不是 generic property bag；
3. `Validate` 能独立判断 provider data；
4. native context 保留原生工具的有效上下文；
5. Safe Probe 具有明确、非破坏性的最小检查，且不夸大证据强度；
6. 能复用既有 Link、Route、Resolve 和 Observation 契约，不引入机制专属分支；
7. 不附带安装器、credential store、通用 session manager 或万能 Execute。

一个真实路径包含某种工具，并不自动要求新增 Provider。只有该步骤确实拥有独立的校验、原生上下文或安全测量契约时，才形成新的 Provider 边界；当前评估结果与实现清单属于[当前实现快照](../current-architecture.md)。

## 后续实例化与执行

当前 Provider 契约不提供通用 Execute。未来 Plan / Instance 层需要建立 tunnel、session、数据库连接或部署操作时，仍由对应 Provider 定义窄而原生的 instantiate / execute / supervise / teardown 操作：

- 操作输入输出必须具体，能够表达该机制真实生命周期；
- 前一步产生的 endpoint、session 或其他 runtime output 通过 Instance 显式传递；
- 临时端口、token、SQL、进程句柄和运行状态不写回 Declaration；
- 原生权限、事务、错误、取消和资源释放语义不得被通用接口抹平；
- 执行产生的 Observation 与 Safe Probe evidence 必须可区分。

这允许后续层复用同一 Provider 边界，同时避免为当前 Resolve 提前加入虚假的万能执行模型。
