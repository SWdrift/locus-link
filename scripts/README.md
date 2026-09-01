# Scripts

所有用户入口均为无参数 PowerShell 脚本。产物边界见[产物设计](../documents/design/产物设计.md)。

| 脚本 | 作用 |
| --- | --- |
| `build.ps1` | 构建内嵌 Web UI 的 `temp/bin/locus.exe`。 |
| `build-backend.ps1` | 构建不含 Web UI 的 `temp/bin/locus-backend.exe`。 |
| `build-web-ui.ps1` | 构建 `internal/web/ui/dist/`。 |
| `start-web.ps1` | 构建并启动完整内嵌形态。 |
| `start-web-api.ps1` | 构建并启动不含 Web UI 的 Go 后端。 |
| `start-web-ui.ps1` | 仅启动 Vite 前端，代理本机 Web API。 |
| `test-e2e.ps1` | 运行 workspace E2E，并保留 `temp/e2e-run/` fixture。 |
| `verify.ps1` | 运行 Web UI 构建、Go 测试、E2E 和 Markdown 链接检查。 |

`internal/` 保存上述入口复用的实现脚本，不作为直接入口。
