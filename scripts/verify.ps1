$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$oldLocation = Get-Location

function Invoke-VerificationStep {
    param(
        [string]$Name,
        [scriptblock]$Command
    )

    Write-Host "`n==> $Name"
    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "$Name failed with exit code $LASTEXITCODE"
    }
}

try {
    Set-Location $repo
    Invoke-VerificationStep 'Web UI build' { & (Join-Path $repo 'scripts/build-web-ui.ps1') }

    Invoke-VerificationStep 'Go tests and workspace E2E' { go test ./... }
    Invoke-VerificationStep 'Markdown links' { pnpm --dir .tools/markdown run check:links }

    Write-Host "`nVerification passed. E2E artifacts retained at $(Join-Path $repo 'temp/e2e-run')"
}
finally {
    Set-Location $oldLocation
}
