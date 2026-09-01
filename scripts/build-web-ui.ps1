$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$ui = Join-Path $repo 'internal/web/ui'
$pnpmStore = Join-Path $repo 'temp/.pnpm-store'
$exitCode = 1
$pnpmCommand = Get-Command pnpm -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
if ($null -eq $pnpmCommand) {
    throw 'pnpm executable was not found on PATH.'
}

& $pnpmCommand.Source "--config.store-dir=$pnpmStore" --dir $ui run build
$exitCode = $LASTEXITCODE
if ($exitCode -ne 0) { exit $exitCode }
