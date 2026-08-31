# locus-link

locus-link 是一个 Agent 优先的 Link 管理工具。它把机器、连接方式和 Route 统一声明和管理，让用户与 Agent 都能用同一种方式查询“从哪里、通过什么方式，可以到达哪个目标”。

Agent 可以根据当前位置和所需能力找到匹配的 Route，取得执行顺序、底层工具所需参数和最近一次探测结果，再调用相应工具完成后续自动化。locus-link 本身只负责解析 Route 和检查连接是否可用，不会替你启动 FRP、建立 SSH session 或执行 Salt operation，也不保存 Secret。

## Quick Start

在包含 `.locus/registry` 的项目或其任意子目录中运行：

```text
locus resolve production-host --capability shell --from workstation.dev-a --vantage office-lan
```

`resolve` 返回输入 Binding 解释、canonical target Entity facts、matched Route、ordered Links、Provider 原生上下文、documentation references 和已有 evidence。需要刷新现实证据时：

```text
locus probe route.prod-shell --from workstation.dev-a --vantage office-lan
locus resolve production-host --capability shell --from workstation.dev-a --vantage office-lan
```

正常循环是：

```text
resolve → probe（需要时）→ resolve
```

`resolve` 只读取 world model 和已有 Observation，不调用 Probe、不写 Observation。`probe` 执行安全检查并写入 Link Observation，不执行 Route 的 operational action。

- `from`：当前 operational Entity / actor；
- `vantage`：reachability Observation 成立的网络观察位置。

## Commands

| 命令 | 用途 |
|---|---|
| `locus resolve <target> --capability <name>` | 解析匹配的显式 Route 和当前 evidence |
| `locus probe <link-or-route-id>` | 安全探测 Link 或 Route |
| `locus context` | 查看当前 Scope、imports、Binding、actor 和 vantage |
| `locus show <ref-or-id>` | 查看声明对象和引用解析 |
| `locus list [entity\|link\|route]` | 列出 Registry 声明 |
| `locus status [link-or-route-id]` | 查看详细 evidence |
| `locus web` | 启动仅监听本机回环地址的 Web UI |

完整 flags、输出、退出码和副作用见 [CLI 公共契约](documents/design/contracts/CLI契约.md)；Web HTTP 接口和安全边界见[本机 Web 契约](documents/design/contracts/Web契约.md)。

## Registry

创建并校验 Registry：

```text
locus init --scope-kind project --scope-id project.example
locus validate
```

`locus` 默认从当前目录向上寻找 `.locus/registry/scope.yaml`；`--registry` 用于 override、automation、tests 和特殊多 Registry 场景。

Registry 使用 YAML 声明 Scope、Import、Binding、Entity、Link 和人工排序的 Route。完整 schema、identity、引用规则和 Provider data 见[声明 YAML 公共契约](documents/design/contracts/声明契约.md)。

## Build and Verification

需要 Go 1.26 或兼容版本。

PowerShell：

```powershell
New-Item -ItemType Directory -Force temp/bin | Out-Null
go build -o temp/bin/locus.exe ./cmd/locus
./test/reproduce.ps1
```

POSIX shell：

```sh
mkdir -p temp/bin
go build -o temp/bin/locus ./cmd/locus
go test ./...
```

E2E declarations 和 simulated device state 位于 `test/e2e/case/`；完整运行产物保留在 `temp/e2e-run/`。

## Documentation

- [设计文档入口](documents/design/README.md)
- [CLI 公共契约](documents/design/contracts/CLI契约.md)
- [声明 YAML 公共契约](documents/design/contracts/声明契约.md)
- [本机 Web 契约](documents/design/contracts/Web契约.md)
- [当前实现快照](documents/current-architecture.md)
