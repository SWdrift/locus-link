# AGENTS.md

## 快速指令

在仓库根目录使用 PowerShell 运行。脚本快捷入口及职责见 [`scripts/README.md`](scripts/README.md)；带 Web UI 与不带 Web UI 的构建边界见[产物设计](documents/design/产物设计.md)。

- 完整验证：`.\scripts\verify.ps1`
- Workspace E2E：`.\scripts\test-e2e.ps1`
- Web 联调：分别在两个终端运行 `.\scripts\start-web-api.ps1` 和 `.\scripts\start-web-ui.ps1`。该方式跳过内嵌 Web UI 构建，适合快速验证 Web 端更改。
- 使用 pwsh 而不是 powershell。

## 工作区安全

- 所有读取、写入、生成文件、子进程工作目录、fixtures 和测试状态都必须保留在本仓库工作区内。
- pnpm、npm 等 Node 包管理器缓存不属于工作区产物，必须沿用机器级全局配置；不得通过脚本、环境变量或配置文件把 store/cache 重定向到仓库内（包括 `temp/.pnpm-store`、`temp/.npm-cache`、`internal/temp/`）。
- 测试不得读取或修改工作区外的用户配置、凭据、服务、网络状态或文件。
- 外部工具行为必须通过在 `temp/` 下创建的确定性辅助可执行文件进行验证；测试不得依赖已安装的 FRP/SSH/Salt 服务。
- 测试使用的观测数据存储必须显式重定向到测试工作区内的路径。
- 可复用的 end-to-end 声明和模拟设备状态必须存放在 `test/e2e/case/` 下；测试运行时将其具现化到 `temp/e2e-run/` 下。
- 测试结束后，`temp/e2e-run/` 下生成的 fixtures、二进制文件、SQLite 状态和解析结果必须保留，以便手动复现；新的测试运行可以用确定性方式替换该目录。

## 验证

- 验证必须匹配改动范围，并优先使用 `scripts/` 下的用户入口；不得直接调用 `scripts/internal/` 下的实现脚本。
- Web UI 或 Web API 更改在开发期间可以使用 `start-web-api.ps1` 和 `start-web-ui.ps1` 启动前后端，并在实际 Web 页面中验证受影响流程。后端代码更改后必须重启 `start-web-api.ps1`。
- CLI 流程、声明、Scope、注册机制、运行模型或跨层集成发生变化时，必须运行 `.\scripts\test-e2e.ps1`。测试失败暴露实现或设计缺陷时，修复后必须重新运行。
- 非纯文档改动完成前必须运行 `.\scripts\verify.ps1`，统一验证 Web UI 构建、Go 测试、workspace E2E 和 Markdown 链接。
- 仅修改 Markdown 文档时，可以只运行 `pnpm --dir .tools/markdown run check:links`，校验仓库内本地文件链接与标题锚点。

## 格式化

- internal\web\ui 可使用 `pnpm --dir internal\web\ui run format` 格式化。
