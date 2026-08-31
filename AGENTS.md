# AGENTS.md

## 快速指令

在仓库根目录使用 PowerShell 运行：

| 指令 | 用途 |
| --- | --- |
| `./scripts/build.ps1` | 构建 CLI 到 `temp/bin/locus.exe`。 |
| `./scripts/test-e2e.ps1` | 运行 workspace end-to-end CLI 流程，并保留 `temp/e2e-run/` 产物。 |
| `./scripts/start-test-web.ps1` | 使用已保留的 E2E 产物启动本地测试 Web UI；可用 `-Refresh` 重建产物、`-NoBrowser` 禁止自动打开浏览器、`-Address 127.0.0.1:<端口>` 更改监听地址。 |
| `./scripts/verify.ps1` | 运行完整 Go 测试、Web UI 构建和 Markdown 链接检查。 |

## 工作区安全

- 所有读取、写入、生成文件、子进程工作目录、fixtures 和测试状态都必须保留在本仓库工作区内。
- 测试不得读取或修改工作区外的用户配置、凭据、服务、网络状态或文件。
- 外部工具行为必须通过在 `temp/` 下创建的确定性辅助可执行文件进行验证；测试不得依赖已安装的 FRP/SSH/Salt 服务。
- 测试使用的观测数据存储必须显式重定向到测试工作区内的路径。
- 可复用的 end-to-end 声明和模拟设备状态必须存放在 `test/e2e/case/` 下；测试运行时将其具现化到 `temp/e2e-run/` 下。
- 测试结束后，`temp/e2e-run/` 下生成的 fixtures、二进制文件、SQLite 状态和解析结果必须保留，以便手动复现；新的测试运行可以用确定性方式替换该目录。

## 验证

- 完成工作的前提是工作区本地的 end-to-end CLI 流程通过。
- End-to-end fixtures 可以使用 `temp/` 下的目录来模拟项目、环境和设备。
- 当完整流程测试暴露 Scope、注册机制或模型缺陷时，必须更新实现或设计，并重新运行完整流程。
- 第一个 vertical slice 最多进行三轮完整的实现/测试迭代。
- 日常修改或新增 Markdown 文档后，必须运行 `pnpm --dir .tools/markdown run check:links`，校验仓库内本地文件链接与标题锚点。


