$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$output = Join-Path $repo 'temp/bin/locus.exe'
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

    New-Item -ItemType Directory -Force (Split-Path -Parent $output) | Out-Null
    go build -o $output ./cmd/locus
    $exitCode = $LASTEXITCODE
    if ($exitCode -eq 0) {
        Write-Host "Built $output"
    }
}
finally {
    Set-Location $oldLocation
    $env:GOCACHE = $oldGoCache
    $env:GOMODCACHE = $oldGoModCache
    $env:GOPATH = $oldGoPath
}

if ($exitCode -ne 0) { exit $exitCode }
