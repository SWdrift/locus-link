# locus-link 与 MCP

locus-link 是 Locus 项目下的一个分支，与 MCP 处理不同层次的问题。

MCP 负责 Agent 与外部能力之间的标准通信，包括能力发现、调用和上下文提供。数据库、Git、文件系统、Shell 等都可以通过 MCP 暴露；Registry、Gateway、认证和工具管理也属于这一层。

locus-link 不重复这些能力。它描述具体环境中的对象、关系、项目绑定、已知访问方式、运行现场和观测结果，并据此解析某个目标在当前环境中应如何被访问和使用。

因此可以简单理解为：

```text
MCP：提供能力
locus-link：解析环境并绑定能力
```

locus-link 的 `Entity / Binding / Link / Route / Context / Observation` 描述 concrete environment；MCP 的 Server、Tool、Resource 描述可供 Agent 使用的 capability。两套模型保持独立。

MCP 可以同时出现在 locus-link 两侧：

```text
Agent
  │
  ├─ MCP → locus-link
  └─ MCP → DB / Git / Filesystem / ...

locus-link
  │
  └─ resolve environment
       └─ bind to MCP or native provider
```

因此 locus-link Core 不依赖 MCP。CLI、WebUI 和 MCP 都只是 Core 的接口；MCP Registry、Gateway、认证、Secret 管理和通用 Tool Routing 不属于 locus-link 的职责。

locus-link 的 `resolve` 也不负责通用工具搜索。工具系统回答“什么能力可以完成这件事”，locus-link 回答“在这个具体环境中，目标是谁、当前如何到达它、已有哪种方法，以及最终应使用什么能力”。

locus-link 是否有独立价值，取决于它能否让不了解现场的 Agent 直接复用已有 operational knowledge，而不必重新阅读网络配置、CI 脚本和人工说明。如果只是重新包装普通配置文件，则没有必要作为 Locus 项目下的独立分支存在。
