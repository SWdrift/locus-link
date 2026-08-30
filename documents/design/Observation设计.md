# Locus Link Observation 设计

## 简述

Observation 是某条已声明 Link 在特定 vantage、特定时间的一次现实证据。它与 Declaration 和 Instance 分离，以只追加方式记录，并在查询时按 freshness 解释；Route evidence 由 constituent Link observations 动态聚合，不单独持久化。

本文定义稳定的证据语义和 Store 能力，不固定当前存储技术或默认路径。当前 SQLite 实现、状态路径和已实现字段见[当前实现快照](../current-architecture.md)；Safe Probe 的测量边界见[Provider 设计](Provider设计.md)，Resolve 的候选与结果契约见[声明与解析设计](声明与解析设计.md)。

## 职责

本文负责：

- 定义以 canonical Link 为 subject 的 Observation；
- 定义 vantage、时间、freshness 和证据适用范围；
- 定义 append/query、Probe 记录和 Resolve 读取语义；
- 定义 Route evidence 聚合与证据强度；
- 划清 Safe Probe evidence 与未来 execution observation 的边界。

本文不负责：

- 定义 Provider 应如何测量具体外部机制；
- 为 Entity、Route 或 capability 建立独立持久化 Observation；
- 固定 SQLite schema、操作系统路径或 Store 部署形态；
- 使用 evidence 自动选择 Route 或修改 Declaration。

## 证据模型

```text
Declared Link
+ Runtime vantage
+ latest applicable Observation
→ Link evidence

ordered Link evidence
→ Route evidence
```

Declaration 表达维护过的已知路径；Observation 表达某次实际测量；Instance 表达某次实例化或执行中的参数与资源。它们生命周期不同：

- failure 或 timeout 不删除 Link；
- success 不修改 Link、Route 或 Provider data；
- 临时端口、token、SQL、session 和进程句柄不因被观察而成为 Declaration；
- Observation 不把推断自动写回 Registry；
- Route evidence 随 constituent Link observations 的查询结果动态变化。

## Observation 记录

一条 Observation 至少具有：

```yaml
id: <observation-id>
subject: project.example::link.prod-ssh
vantage: office-lan
status: success
observed_at: 2026-08-30T10:15:00Z
expires_at: 2026-08-30T10:30:00Z
provider: ssh
evidence:
  kind: tcp-endpoint
error: null
```

规则：

- `subject` 必须是 canonical Link ID；
- `vantage` 必填，并与 actor Entity 分开表达；
- `status` 为 `success | failure | unknown`；
- `observed_at` 必填，`expires_at` 可选；
- `provider` 与 `evidence.kind` 标识证据来源和实际测量种类；
- evidence 必须说明实际测得范围，不得只给出含糊的“可用”结论；
- evidence 与 error 在进入 Store 前完成脱敏；
- 记录只追加，不原地覆盖历史，也不更新 Declaration 或 Instance。

Entity、Route 和 capability 不保存独立 Observation。Route evidence 是查询结果，没有持久化 subject 或 observation ID。

## Vantage 与时间

Vantage 是 reachability evidence 成立的观察位置，不等同于发起调用的 actor Entity。相同 actor 可以在多个 vantage 下得到彼此独立的结果。

只有 canonical subject 与 vantage 都匹配的记录才能作为当前 Link evidence。其他 vantage 的记录可以用于 diagnostics，但不得参与当前 Route 聚合。未显式提供 vantage 时如何形成 host-specific fallback 属于 Runtime Context 的当前实现策略，见[当前实现快照](../current-architecture.md)；fallback 不得产生全局证据。

同一 subject 与 vantage 内，以最新 Observation 解释 freshness：

- 没有记录：`unknown`；
- 最新记录存在且未过 `expires_at`：fresh `success`、`failure` 或 `unknown`；
- 最新记录已过 `expires_at`：`stale`，同时保留其原始 status 和时间供诊断；
- 没有 `expires_at`：记录没有显式到期时间，不得由 Store 擅自套用另一 vantage 或 subject 的期限。

“最新”的稳定排序必须能够处理相同时间戳；具体 ID 与存储排序字段可以演进，但同一组记录的查询结果必须确定。

## Store 契约

Store 的稳定能力只有：

- 追加一条已脱敏的 Link Observation；
- 按 canonical Link subject 与 vantage 查询最新记录；
- 按 subject 查询各 vantage 的最新记录供 diagnostics；
- 保存、比较并返回时间和 evidence 字段；
- 保留历史记录。

存储实现损坏或不可读必须显式报系统错误，不能伪装成 `unknown`。写入失败也不能报告为 Probe success。SQLite、状态文件位置、环境变量和迁移方式均是实现事实，统一记录在[当前实现快照](../current-architecture.md)。

## Probe 记录

Provider 返回已脱敏的 Safe Probe 结果后，由 Probe orchestration 追加 Observation：

- Link Probe 为实际尝试的 Link 追加一条记录；
- Route Probe 按声明顺序尝试 Link；
- 每个实际尝试的 Link 都追加记录；
- 前序 failure 后停止，未尝试的 Link 不写记录；
- 整体 Probe 失败不使已完成的测量失效；
- 不创建 Route Observation。

Provider 启动失败、声明非法等系统错误是否形成 Observation，必须由明确的测量边界决定；不得把“没有执行测量”伪装为 Link failure。Probe 的用户可见副作用和失败展示由[CLI 设计](CLI设计.md)定义。

## Resolve 读取

Resolve 对每个 canonical Link，以当前 vantage 查询最新 Observation，形成 Link evidence，再聚合 Route evidence。读取是无副作用的，不触发 Probe、不追加记录。

Evidence 与 Route candidate selection 正交：

- fresh success 不提升排名；
- fresh failure 不移除候选；
- stale 不等于当前 failure；
- unknown 不阻止声明匹配；
- resolved / unresolved / ambiguous 只由[声明与解析设计](声明与解析设计.md)定义的 target、capability、applicability 和 cardinality 决定。

## Route evidence 聚合

对 Route 的全部 constituent Links：

- 任一 Link 有适用的 fresh failure：`failure`；
- 全部 Link 都有适用的 fresh success：`success`；
- 每条 Link 都有当前 vantage 的记录、没有 fresh failure，且至少一条最新记录 stale：`stale`；
- 其他情况：`unknown`。

聚合只使用声明 Route 中的 Link，不跨 vantage 补齐缺失记录，也不把未探测步骤虚构为 failure。聚合结果应同时保留每条 Link evidence，使调用方能够看到结论来源。

## 证据强度

Observation 只证明其 `evidence.kind` 描述的事实。Safe Probe success 可能只说明配置可解析、endpoint 可达或原生 ping 成功；Route evidence `success` 只是这些 Link-level Safe Probe results 的聚合，不承诺：

- credential 有效；
- 认证或业务权限成立；
- 完整 capability 可以执行；
- Route 已经实例化；
- 后续执行不会失败。

错误摘要和证据不得包含 Secret、credential value、完整敏感配置或未脱敏进程输出。Provider failure 是测量结果；Store、序列化或进程控制故障是系统错误，二者必须可区分。

## 未来执行 Observation

未来实例化与执行可以追加更强的现实证据，但不得复用 Safe Probe 的 `evidence.kind` 冒充同等含义。扩展时必须显式记录 observation source、operation kind、subject、vantage / execution context、时间和实际证明范围。

Execution observation 仍遵守以下边界：

- 只追加，不覆盖 Declaration 或 Probe 历史；
- Instance 的临时值保留在 Instance，不提升为稳定声明；
- 一次执行成功不自动证明未来执行，也不改变 Route cardinality；
- 若证据 subject 不再是 Link，必须先建立独立、明确的查询与聚合契约，不能把 Route 或 Instance 状态塞入现有 Link Observation。

相关验证样例统一由[测试设计](测试设计.md)维护。
