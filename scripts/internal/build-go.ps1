param(
    [Parameter(Mandatory = $true)]
    [string]$Target,
    [Parameter(Mandatory = $true)]
    [string]$Output,
    [Parameter(Mandatory = $true)]
    [ValidateSet('complete', 'backend')]
    [string]$Artifact
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$oldLocation = Get-Location
$exitCode = 1

try {
    $versionPath = Join-Path $repo 'VERSION'
    $version = (Get-Content -LiteralPath $versionPath -Raw).Trim()
    if ($version -notmatch '^\d+\.\d+\.\d+$') {
        throw "VERSION must contain a semantic version, got '$version'."
    }
    $commit = 'unknown'
    $gitCommand = Get-Command git -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $gitCommand) {
        $resolvedCommit = (& $gitCommand.Source -C $repo rev-parse HEAD 2>$null)
        if ($LASTEXITCODE -eq 0 -and $resolvedCommit) {
            $commit = $resolvedCommit.Trim()
            $changes = (& $gitCommand.Source -C $repo status --porcelain --untracked-files=normal 2>$null)
            if ($LASTEXITCODE -eq 0 -and $changes) {
                $commit += '-dirty'
            }
        }
    }
    $ldflags = "-X locus-link/internal/buildinfo.Version=$version -X locus-link/internal/buildinfo.Commit=$commit -X locus-link/internal/buildinfo.Artifact=$Artifact"
    Set-Location $repo

    New-Item -ItemType Directory -Force (Split-Path -Parent $Output) | Out-Null
    go build -ldflags $ldflags -o $Output $Target
    $exitCode = $LASTEXITCODE
    if ($exitCode -eq 0) {
        Write-Host "Built $Output"
    }
}
finally {
    Set-Location $oldLocation
}

if ($exitCode -ne 0) { exit $exitCode }
