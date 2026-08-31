# Locus Link 公共契约

本目录维护用户、Agent、Registry 作者和自动化可以直接依赖的公共接口。公共契约强调具体、可验证和兼容性稳定；内部 Go 类型、SQLite schema、代码文件和组件拆分不属于本目录。

## 契约列表

1. [CLI 契约](CLI契约.md)：命令、参数、输出、副作用、错误和退出码。
2. [声明契约](声明契约.md)：Registry 布局、YAML schema、identity、引用、Provider data 和完整声明样例。
3. [本机 Web 契约](Web契约.md)：loopback HTTP API、Graph/Status/Knowledge、Resolve、Probe 和安全边界。

## 稳定性边界

公共契约包括：

- `locus` 命令、flags、退出码和可脚本化 JSON 的字段语义；
- 本机 `/api/v0` 路径、响应语义、副作用和访问安全边界；
- `scope.yaml` 与 Entity、Link、Route YAML 的字段和引用语义；
- `api_version`、严格解码、静态校验和 Secret 不泄漏保证；
- 用户需要维护的 Provider-specific `provider_data`。

公共契约不包括：

- Go package、struct、interface 或函数名；
- Registry loader、Resolver、Provider adapter 和 Store 的内部拆分；
- SQLite 表、列、索引、迁移和进程实现；
- 当前支持范围、已知偏差和实施进度。

## 兼容规则

- YAML 字段删除、字段含义改变或引用语义改变需要新的 `api_version`；
- 新增可选字段必须保持旧声明有效；新增必填字段属于破坏性变更；
- CLI 与 Web API JSON 已有字段的含义和类型不得静默改变；退出码或 HTTP 副作用含义不得复用；
- 新命令、新 API endpoint、新 Provider 或新可选输出可以兼容增加，但必须同步契约和测试；
- 内部实现可以原子调整，只要公共契约和可观察行为保持不变。

当前实现对公共契约的覆盖与偏差见[当前实现快照](../../current-architecture.md)，验证场景见[测试设计](../测试设计.md)。
