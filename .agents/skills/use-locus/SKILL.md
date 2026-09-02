---
name: use-locus
description: "使用已安装的 locus CLI 发现、维护并使用当前环境中的 Entity、Link、Route、Binding 与 evidence。适用于查找连接方式、访问目标、描述或修正环境拓扑、检查连接状态，或明确要求使用 locus。"
---

# Use Locus

用户可以直接用自然语言描述目标或连接方式，不要求其使用 Locus 术语。

默认流程：

```text
context
→ inspect existing declarations
→ reconcile user-provided knowledge
→ register stable knowledge when safe
→ resolve
→ probe if needed
→ use native tools
```

先执行：

```text
locus context --json
```

默认使用当前工作目录发现的 Registry；项目 `.locus/registry` 优先，否则使用用户 Registry。查询命令默认加 `--json`。

需要访问某个目标时优先：

```text
locus resolve <target> <capability> --json
```

必要时结合：

```text
locus show <ref-or-id> --json
locus list entity --json
locus list link --json
locus list route --json
locus graph --json
locus status [<id>] --json
```

如果用户提供了新的稳定环境知识，例如新的主机、连接方式、Route、Binding 或旧路径替换：

1. 先与现有声明对齐；
2. 已存在则复用；
3. 兼容的新信息可以补充；
4. 用户明确说明旧信息失效时可以更新；
5. identity 或语义存在未解决冲突时才询问用户；
6. 信息足够且无冲突时，可直接维护当前 Registry，无需再次要求用户确认“写入 Locus”；
7. 修改后执行：

```text
locus validate --json
```

不要仅凭名称、IP、端口或相似描述合并两个 Entity。

只登记稳定、可复用的信息。临时 session、一次性端口、token、Secret value 和瞬时运行状态不进入 Registry。

Entity、Link 和 Route 的语义保持开放。连接方式可以是 SSH、FRP、Salt、数据库客户端、内部脚本，也可以是人工流程，例如：

```text
询问某个人如何连接
按文档操作
等待人工审批
```

如果现有 Provider 无法表达某种机制，不要虚构 Provider；保留可表达的 Entity、documentation 和上下文，并指出模型缺口。

`resolve` 只解析声明，不执行 Route。拿到 Route 后读取相关 documentation，并使用 SSH、FRP、Salt、数据库客户端等原生工具完成真实操作。

只有当前任务需要新 evidence 时才执行：

```text
locus probe <link-or-route-id> --timeout <duration> --json
```

Probe 后重新执行 `status` 或 `resolve`，不要手工判断 Observation 是否适用。

如果 `completeness != complete`，不得把未发现对象或 Route 断言为不存在。

`ambiguous` 时，若当前用户输入已经明确指定某条路径或机制，可直接据此选择；否则报告候选差异并询问用户。

不要输出、记录或写入 Secret value。
