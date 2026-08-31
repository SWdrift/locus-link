$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$oldLocation = Get-Location
$oldGoCache = $env:GOCACHE
$oldGoModCache = $env:GOMODCACHE
$oldGoPath = $env:GOPATH
$exitCode = 1

try {
    $env:GOCACHE = Join-Path $repo 'temp/.go-cache'
    $env:GOMODCACHE = Join-Path $repo 'temp/.go-mod-cache'
    $env:GOPATH = Join-Path $repo 'temp/.go-path'
    Set-Location $repo

    go test ./test -run '^TestWorkspaceEndToEnd$' -v
    $exitCode = $LASTEXITCODE
    if ($exitCode -eq 0) {
        Write-Host "Artifacts retained at $(Join-Path $repo 'temp/e2e-run')"
    }
}
finally {
    Set-Location $oldLocation
    $env:GOCACHE = $oldGoCache
    $env:GOMODCACHE = $oldGoModCache
    $env:GOPATH = $oldGoPath
}

if ($exitCode -ne 0) { exit $exitCode }
