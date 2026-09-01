$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$oldLocation = Get-Location
$oldGoCache = $env:GOCACHE
$oldGoModCache = $env:GOMODCACHE
$oldGoPath = $env:GOPATH
$exitCode = 1
$goCommand = Get-Command go -CommandType Application -ErrorAction SilentlyContinue
if ($null -ne $goCommand) {
    $goExecutable = $goCommand.Source
}
else {
    $goExecutable = Join-Path $env:ProgramFiles 'Go/bin/go.exe'
    if (-not (Test-Path -LiteralPath $goExecutable)) {
        throw 'Go executable was not found on PATH or at %ProgramFiles%\Go\bin\go.exe.'
    }
}


try {
    $env:GOCACHE = Join-Path $repo 'temp/.go-cache'
    $env:GOMODCACHE = Join-Path $repo 'temp/.go-mod-cache'
    $env:GOPATH = Join-Path $repo 'temp/.go-path'
    Set-Location $repo
    & (Join-Path $repo 'scripts/build-web-ui.ps1')
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    Remove-Item -LiteralPath (Join-Path $repo 'temp/e2e-run') -Recurse -Force -ErrorAction SilentlyContinue

    & $goExecutable test ./test -run 'EndToEnd$' -v
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
