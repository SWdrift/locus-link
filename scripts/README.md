# Scripts

普通构建与调试入口是无参数 PowerShell 脚本；高影响部署和迁移入口接受显式路径覆盖，并通过 PowerShell `ShouldProcess` 默认请求确认。产物、版本和回滚边界见[产物设计](../documents/design/产物设计.md)。

| 脚本 | 作用 |
| --- | --- |
| `build.ps1` | 构建内嵌 Web UI 的 `temp/bin/locus.exe`。 |
| `build-backend.ps1` | 构建不含 Web UI 的 `temp/bin/locus-backend.exe`。 |
| `build-web-ui.ps1` | 构建 `internal/web/ui/dist/`。 |
| `deploy.ps1` | 构建完整版本、迁移本机 State、备份并部署到每用户安装目录。 |
| `migrate.ps1` | 独立备份并迁移上一版本本机 State DB。 |
| `start-web.ps1` | 构建并启动完整内嵌形态。 |
| `start-web-api.ps1` | 构建并启动不含 Web UI 的 Go 后端。 |
| `start-web-ui.ps1` | 仅启动 Vite 前端，代理本机 Web API。 |
| `test-e2e.ps1` | 运行 workspace E2E，并保留 `temp/e2e-run/` fixture。 |
| `verify.ps1` | 运行 Web UI 构建、Go 测试、E2E 和 Markdown 链接检查。 |
| `build.sh` / `build-backend.sh` | Linux 完整产物与 backend 构建，输出无 `.exe` 后缀。 |
| `build-web-ui.sh` | Linux Web UI 类型检查与 lockfile 构建。 |
| `test-e2e.sh` | Linux workspace E2E。 |
| `verify.sh` | Linux Web UI、Go、E2E 和 Markdown links 完整验证。 |

`start-web.ps1` 与 `start-web-api.ps1` 使用 `test-e2e.ps1` 保留的 `temp/e2e-run/native/` 作为调试运行时，并显式设置其中的用户级 `LOCUS_HOME`、项目注册状态、Observation Store 与模拟设备。首次启动或 fixture 变更后先运行 `test-e2e.ps1`；`start-web-ui.ps1` 通过 `127.0.0.1:7070` 消费该 API。

`internal/` 保存上述入口复用的实现脚本，不作为直接入口。

Windows 使用 `.ps1` 入口并生成 `.exe`；Linux 使用同名 `.sh` 入口并生成 `locus`、`locus-backend`。两端共享 Go targets、`VERSION`、ldflags metadata、pnpm lockfile 与 `go test ./...` 验证边界，不维护平台特有产品逻辑。

## 部署

默认部署：

```powershell
.\scripts\deploy.ps1
```

脚本默认使用用户 Locus 根 `%USERPROFILE%\.locus\`：完整 executable 位于 `bin\locus.exe`，发布清单位于 `release.json`，State 位于 `state\state.db`，备份位于 `backups\`。`LOCUS_HOME` 和 `LOCUS_STATE_PATH` 可覆盖对应默认值。脚本不会修改 `PATH`，会拒绝未知版本、跨多版本升级和正在从安装目录运行的 `locus.exe`。

部署始终先运行 `build.ps1` 生成完整产物，再调用 `migrate.ps1`。迁移或安装失败时恢复 State、旧 executable 和 `release.json`。高影响写操作默认要求确认；`-WhatIf` 可查看目标。只有测试或自动化已把全部路径定向到受控目录时才可使用 `-Confirm:$false`：

```powershell
.\scripts\deploy.ps1 `
  -LocusRoot .\temp\deploy-test\.locus `
  -StatePath .\temp\deploy-test\.locus\state\state.db `
  -BackupRoot .\temp\deploy-test\.locus\backups `
  -Confirm:$false
```

## 迁移

正常升级不需单独调用迁移；部署入口已经调用它。需要预先迁移或诊断时：

```powershell
.\scripts\migrate.ps1
```

`migrate.ps1` 默认构建最新完整产物，也可通过 `-Executable` 使用已经构建的完整 `locus.exe`。迁移只接受上一 State schema、当前 schema 或空数据库；它不迁移 Registry YAML。
