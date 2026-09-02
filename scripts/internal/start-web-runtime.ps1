param(
    [Parameter(Mandatory = $true)]
    [string]$Executable
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$runRoot = Join-Path $repo 'temp/e2e-run/native'
$locusHome = Join-Path $runRoot 'home'
$project = Join-Path $runRoot 'projects/alpha'
$binding = Join-Path $runRoot 'mechanisms/workstation-a.yaml'
$state = Join-Path $runRoot 'state/state.db'
$device = Join-Path $runRoot 'devices/dev-a'
$probeLog = Join-Path $runRoot 'probe-invocations.log'
$required = @(
    $Executable,
    (Join-Path $project '.locus/registry/scope.yaml'),
    (Join-Path $locusHome 'registry/scope.yaml'),
    $binding,
    $state,
    $device
)
$missing = @($required | Where-Object { -not (Test-Path -LiteralPath $_) })
if ($missing.Count -gt 0) {
    throw "Web debug prerequisites are missing: $($missing -join ', '). Run ./scripts/test-e2e.ps1 first."
}

$oldLocation = Get-Location
$oldHome = $env:LOCUS_HOME
$oldStatePath = $env:LOCUS_STATE_PATH
$oldSimRoot = $env:LOCUS_SIM_ROOT
$oldSimLog = $env:LOCUS_SIM_LOG
$oldPath = $env:PATH
$exitCode = 1

try {
    $env:LOCUS_HOME = $locusHome
    $env:LOCUS_STATE_PATH = $state
    $env:LOCUS_SIM_ROOT = $device
    $env:LOCUS_SIM_LOG = $probeLog
    $env:PATH = "$(Join-Path $runRoot 'bin');$oldPath"
    Set-Location $project

    & $Executable ui --from workstation.dev-a --vantage office-lan --mechanism-bindings $binding --address 127.0.0.1:7070
    $exitCode = $LASTEXITCODE
}
finally {
    Set-Location $oldLocation
    $env:LOCUS_HOME = $oldHome
    $env:LOCUS_STATE_PATH = $oldStatePath
    $env:LOCUS_SIM_ROOT = $oldSimRoot
    $env:LOCUS_SIM_LOG = $oldSimLog
    $env:PATH = $oldPath
}

if ($exitCode -ne 0) { exit $exitCode }
