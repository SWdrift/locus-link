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
| `locus context` | 查看根来源、Scope imports、Binding、Source cache、actor 和 vantage |
| `locus graph` | 查看递归 Scope 图、provenance、声明和 partial diagnostics |
| `locus show <ref-or-id>` | 查看声明对象和引用解析 |
| `locus list [binding\|entity\|link\|route]` | 列出 Registry 声明 |
| `locus status [link-or-route-id]` | 查看详细 evidence |
| `locus init --scope-id <id>` / `locus user init --scope-id <id>` | 创建项目或用户 Registry |
| `locus project register\|unregister\|list` | 管理项目反向登记 |
| `locus refresh [alias-path]` | 显式获取、校验并激活 remote Source |
| `locus web` | 启动仅监听本机回环地址的 Web UI |

完整 flags、输出、退出码和副作用见 [CLI 公共契约](documents/design/contracts/CLI契约.md)；Web HTTP 接口和安全边界见[本机 Web 契约](documents/design/contracts/Web契约.md)。

## Registry

创建并校验 Registry：

```text
locus init --scope-id project.example --import-user user --register
locus validate
```

`locus` 默认从当前目录向上寻找 `.locus/registry/scope.yaml`；找不到项目 Registry 时使用 `${LOCUS_HOME}/registry` 用户根。`--registry` 用于 override、automation、tests 和特殊多 Registry 场景。

Registry 使用严格 `locus/v1` YAML 声明 Scope、Import、Binding、Entity、Link 和人工排序的 Route。Import 可递归指向 directory、Git 或 ZIP URL Source；普通读取只使用已激活 remote cache，网络获取仅由 `refresh` 触发。完整 schema、identity、引用规则和 Provider data 见[声明 YAML 公共契约](documents/design/contracts/声明契约.md)。

## Build and Verification

需要 Go 1.26 或兼容版本。

PowerShell：

```powershell
./scripts/build.ps1
./scripts/test-e2e.ps1
./scripts/verify.ps1
```

`build.ps1` 将 CLI 构建到 `temp/bin/locus.exe`；`test-e2e.ps1` 运行 native workspace 与 Scope graph 两个 E2E；`verify.ps1` 依次运行完整 Go 测试（含 E2E）、Web UI 构建和 Markdown 链接检查。脚本使用的 cache 和测试状态均保留在仓库 `temp/` 下。

POSIX shell：

```sh
mkdir -p temp/bin
go build -o temp/bin/locus ./cmd/locus
go test ./...
```

E2E declarations 和 simulated device state 位于 `test/e2e/case/`；native 与 Scope graph 完整运行产物分别保留在 `temp/e2e-run/native/` 和 `temp/e2e-run/scope/`。

Windows 可直接启动保留案例的 Web UI：

```powershell
./scripts/start-test-web.ps1
```

首次运行或需要用当前源码重新生成 E2E 产物时添加 `-Refresh`；脚本启动中文优先的 Project Alpha 页面，按 `Ctrl+C` 停止服务。

## Documentation

- [设计文档入口](documents/design/README.md)
- [CLI 公共契约](documents/design/contracts/CLI契约.md)
- [声明 YAML 公共契约](documents/design/contracts/声明契约.md)
- [本机 Web 契约](documents/design/contracts/Web契约.md)
- [当前实现快照](documents/current-architecture.md)
