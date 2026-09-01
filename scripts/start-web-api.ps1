$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot

& (Join-Path $repo 'scripts/build-backend.ps1')
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

& (Join-Path $repo 'scripts/internal/start-web-runtime.ps1') -Executable (Join-Path $repo 'temp/bin/locus-backend.exe')
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
