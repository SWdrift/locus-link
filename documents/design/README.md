# Locus Link 设计文档

本目录维护已经确定、需要跨任务复用的设计。用户直接依赖的接口集中在公共契约中；系统内部语义、数据流和测试分别由对应设计维护。

## 文档入口

- [产品设计](产品设计.md)：产品问题、Entity / Link / Route 核心语义和长期边界。
- [公共契约](contracts/README.md)：用户、Agent、Registry 作者和自动化可以依赖的稳定接口。
  - [CLI 契约](contracts/CLI契约.md)：命令、flags、JSON、副作用和退出码。
  - [声明契约](contracts/声明契约.md)：Registry、YAML schema、identity、引用和 Provider data。
  - [本机 Web 契约](contracts/Web契约.md)：loopback API、读取投影、Resolve、Probe 和浏览器安全边界。
- [Workspace 与 Registry 设计](Workspace与Registry设计.md)：Locus Home、Scope、Registry Source、项目关联、组合和本地 Observation 归属。
- [系统设计](系统设计.md)：Canonical Declared View、Resolve、Provider、Probe 和 Observation 的内部协作。
- [数据流与存储设计](数据流与存储设计.md)：数据来源、变换、持久化、ownership 和 Secret 血缘。
- [测试设计](测试设计.md)：公共契约和内部不变量的验证场景。
- [当前实现快照](../current-architecture.md)：当前代码、E2E 覆盖和契约偏差。

## 阅读顺序

- 用户和 Registry 作者：产品设计 → 公共契约。
- 实现者：产品设计 → 公共契约 → Workspace 与 Registry 设计 → 系统设计 → 数据流与存储设计 → 测试设计 → 当前实现快照。

## 管理规则

- 公共 YAML、CLI、HTTP API 和 JSON 的破坏性变化必须显式版本化或迁移。
- Go API、SQLite schema 和内部组件拆分不是公共兼容承诺。
- 同一规则只在一份权威文档中完整定义，其他文档使用链接。
- 当前代码盘点和契约偏差只写入 `../current-architecture.md`。
- 任务、进度和待决事项只写入 `cp-locus-link.md`；实现经验写入 `cpm-locus-link.md`。
- 只有出现独立受众、独立生命周期或多份稳定设计时才拆分新文档或子目录。
