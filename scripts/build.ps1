$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot

& (Join-Path $repo 'scripts/build-web-ui.ps1')
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

& (Join-Path $repo 'scripts/internal/build-go.ps1') -Target './cmd/locus' -Output (Join-Path $repo 'temp/bin/locus.exe') -Artifact complete
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
