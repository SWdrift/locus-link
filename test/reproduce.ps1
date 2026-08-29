$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$env:GOCACHE = Join-Path $repo 'temp/.go-cache'
$env:GOMODCACHE = Join-Path $repo 'temp/.go-mod-cache'
$env:GOPATH = Join-Path $repo 'temp/.go-path'
Set-Location $repo
go test ./test -run '^TestWorkspaceEndToEnd$' -v
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Artifacts retained at $(Join-Path $repo 'temp/e2e-run')"
