$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot

& (Join-Path $repo 'scripts/internal/build-go.ps1') -Target './cmd/locus-backend' -Output (Join-Path $repo 'temp/bin/locus-backend.exe') -Artifact backend
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
