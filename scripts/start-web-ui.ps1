$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$ui = Join-Path $repo 'internal/web/ui'

& pnpm --dir $ui run dev '--host' '127.0.0.1' '--port' '5173'
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
