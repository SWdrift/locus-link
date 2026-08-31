# locus-link 设计文档

本目录维护已经确定、需要跨任务复用的设计。用户直接依赖的接口集中在公共契约中；系统内部语义、数据流和测试分别由对应设计维护。

## 文档入口

- [基础核心概念](base-核心概念.md)：统一定义 Entity、Link/Graph、Binding、根 Scope Route overlay、Resolve、Probe、Observation 等名词及其产品生命周期。
- [公共契约](contracts/README.md)：用户、Agent、Registry 作者和自动化可以依赖的稳定接口。
  - [CLI 契约](contracts/CLI契约.md)：命令、flags、JSON、副作用和退出码。
  - [声明契约](contracts/声明契约.md)：Registry、YAML schema、identity、引用和 Provider data。
  - [本机 Web 契约](contracts/Web契约.md)：loopback API、读取投影、Resolve、Probe 和浏览器安全边界。
- [基础 Scope 设计](base-Scope设计.md)：Scope、Registry、Source、显式 import graph、alias、去重、回环阻断、partial diagnostics 和 remote provenance。
- [用户级 Locus 设计](base-用户级Locus设计.md)：用户根 Scope、项目反向登记、remote cache/refresh、Observation Store 与本机状态。
- [基础系统设计](base-系统设计.md)：引用核心概念后，定义知识模型、运行时视图、Core 闭环、机制边界与对外投影的处理不变量。
- [基础数据设计](base-数据设计.md)：declaration/source/context/evidence provenance、持久化和 Secret 边界。
- [测试设计](测试设计.md)：native Core 基线、Scope graph、用户级 Locus、remote cache/refresh 和公共不变量。
- [当前实现快照](../current-architecture.md)：当前代码、E2E 覆盖和设计偏差。

## 阅读顺序

- 用户和 Registry 作者：产品设计 → 公共契约 → Scope 设计 → 用户级 Locus 设计。
- 实现者：产品设计 → 公共契约 → Scope 设计 → 用户级 Locus 设计 → 系统设计 → 数据流与存储设计 → 测试设计 → 当前实现快照。

## 管理规则

- 项目与产品名称统一写作 `locus-link`；`locus` 仅用于 executable、`locus/v0`、`locus://` 和 `locus.*` 协议标识；“用户级 Locus”专指本文定义的用户根入口与本机状态边界。
- 公共 YAML、CLI、HTTP API 和 JSON 的破坏性变化必须显式版本化或迁移。
- Go API、SQLite schema 和内部组件拆分不是公共兼容承诺。
- 同一规则只在一份权威文档中完整定义，其他文档使用链接。
- 当前代码盘点和契约偏差只写入 `../current-architecture.md`。
- 任务、进度和待决事项只写入 `cp-locus-link.md`；实现经验写入 `cpm-locus-link.md`。
- 只有出现独立受众、独立生命周期或多份稳定设计时才拆分新文档或子目录。
