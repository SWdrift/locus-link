# locus-link 公共契约

本目录只维护用户、Agent、Registry 作者和自动化可直接依赖的稳定接口：

- [CLI 契约](CLI契约.md)：命令、flags、JSON、副作用和退出码。
- [声明契约](声明契约.md)：Registry YAML、identity、引用和 Provider data。
- [本机 Web 契约](Web契约.md)：loopback API、Graph/Status/Knowledge、Resolve、Probe 和安全边界。

Go API、内部组件拆分、SQLite schema、当前支持范围和实施进度不属于公共契约。

## 兼容规则

- YAML 字段或引用语义的破坏性变化需要新的 `api_version`；新增必填字段也是破坏性变化。
- CLI/Web JSON 字段、退出码和 HTTP 副作用不得静默改变或复用含义。
- 兼容新增命令、endpoint、Provider 或可选字段时，必须同步契约和测试。
- 内部实现可以原子调整，只要公共行为保持不变。

当前覆盖与偏差见[当前实现快照](../../current-architecture.md)，验证场景见[测试设计](../测试设计.md)。
