[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'High')]
param(
    [string]$StatePath,
    [string]$BackupDir,
    [string]$Executable
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot

if ([string]::IsNullOrWhiteSpace($StatePath)) {
    if ($env:LOCUS_STATE_PATH) {
        $StatePath = $env:LOCUS_STATE_PATH
    }
    else {
        $locusRoot = $env:LOCUS_HOME
        if ([string]::IsNullOrWhiteSpace($locusRoot)) {
            $userHome = [Environment]::GetFolderPath('UserProfile')
            if ([string]::IsNullOrWhiteSpace($userHome)) { throw 'The OS user home directory is unavailable.' }
            $locusRoot = Join-Path $userHome '.locus'
        }
        $StatePath = Join-Path $locusRoot 'state/state.db'
    }
}
$StatePath = [System.IO.Path]::GetFullPath($StatePath)

if ([string]::IsNullOrWhiteSpace($BackupDir)) {
    $BackupDir = Join-Path (Split-Path -Parent $StatePath) 'backups'
}
$BackupDir = [System.IO.Path]::GetFullPath($BackupDir)

if ([string]::IsNullOrWhiteSpace($Executable)) {
    & (Join-Path $repo 'scripts/build.ps1')
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    $Executable = Join-Path $repo 'temp/bin/locus.exe'
}
$Executable = [System.IO.Path]::GetFullPath($Executable)
if (-not (Test-Path -LiteralPath $Executable -PathType Leaf)) {
    throw "Migration executable does not exist: $Executable"
}

$release = (& $Executable version --json | ConvertFrom-Json)
if ($LASTEXITCODE -ne 0) {
    throw "Unable to read migration executable version: $Executable"
}
if ($release.artifact -ne 'complete') {
    throw "Migration requires the complete artifact, got '$($release.artifact)'."
}

$operation = "Migrate state schema to $($release.state_schema_version) with backup in $BackupDir"
if (-not $PSCmdlet.ShouldProcess($StatePath, $operation)) { return }

$output = & $Executable migrate --state $StatePath --backup-dir $BackupDir --json
$exitCode = $LASTEXITCODE
if ($exitCode -ne 0) {
    throw "State migration failed with exit code $exitCode."
}
$output | Write-Output
