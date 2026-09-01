[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'High')]
param(
    [string]$LocusRoot,
    [string]$StatePath,
    [string]$BackupRoot
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
. (Join-Path $repo 'scripts/internal/deploy/release.ps1')

if ([string]::IsNullOrWhiteSpace($LocusRoot)) {
    if ($env:LOCUS_HOME) {
        $LocusRoot = $env:LOCUS_HOME
    }
    else {
        $userHome = [Environment]::GetFolderPath('UserProfile')
        if ([string]::IsNullOrWhiteSpace($userHome)) { throw 'The OS user home directory is unavailable.' }
        $LocusRoot = Join-Path $userHome '.locus'
    }
}
$LocusRoot = [System.IO.Path]::GetFullPath($LocusRoot)
if ([string]::IsNullOrWhiteSpace($StatePath)) {
    $StatePath = if ($env:LOCUS_STATE_PATH) { $env:LOCUS_STATE_PATH } else { Join-Path $LocusRoot 'state/state.db' }
}
if ([string]::IsNullOrWhiteSpace($BackupRoot)) {
    $BackupRoot = Join-Path $LocusRoot 'backups'
}
$StatePath = [System.IO.Path]::GetFullPath($StatePath)
$BackupRoot = [System.IO.Path]::GetFullPath($BackupRoot)

& (Join-Path $repo 'scripts/build.ps1')
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$artifact = Join-Path $repo 'temp/bin/locus.exe'
$targetRelease = (& $artifact version --json | ConvertFrom-Json)
if ($LASTEXITCODE -ne 0) { throw 'Built locus.exe did not return version metadata.' }
if ($targetRelease.artifact -ne 'complete') {
    throw "Deployment requires the complete artifact, got '$($targetRelease.artifact)'."
}

$installedRelease = Get-LocusReleaseMetadata -LocusRoot $LocusRoot
Assert-LocusUpgradePath -Installed $installedRelease -Target $targetRelease
$targetExecutable = Join-Path $LocusRoot 'bin/locus.exe'
Assert-LocusExecutableStopped -Executable $targetExecutable

$fromVersion = if ($null -eq $installedRelease) { 'none' } else { $installedRelease.version }
$operation = "Deploy locus-link $fromVersion -> $($targetRelease.version), migrate state $StatePath, and retain rollback backups"
if (-not $PSCmdlet.ShouldProcess($LocusRoot, $operation)) { return }

$stamp = [DateTime]::UtcNow.ToString('yyyyMMddTHHmmss.fffffffZ')
$backupDir = Join-Path $BackupRoot ("deploy-$fromVersion-to-$($targetRelease.version)-$stamp")
New-Item -ItemType Directory -Force $backupDir | Out-Null
$stateExisted = Test-Path -LiteralPath $StatePath -PathType Leaf
$emptyStateBackup = $null
if ($stateExisted -and (Get-Item -LiteralPath $StatePath).Length -eq 0) {
    $emptyStateBackup = Join-Path $backupDir 'state-empty.db'
    Copy-Item -LiteralPath $StatePath -Destination $emptyStateBackup
}
$hadInstallation = $null -ne $installedRelease
if ($hadInstallation) {
    Copy-Item -LiteralPath $targetExecutable -Destination (Join-Path $backupDir 'locus.exe')
    Copy-Item -LiteralPath (Join-Path $LocusRoot 'release.json') -Destination (Join-Path $backupDir 'release.json')
}

$migrationResult = $null
try {
    $migrationOutput = & (Join-Path $repo 'scripts/migrate.ps1') `
        -StatePath $StatePath `
        -BackupDir $backupDir `
        -Executable $artifact `
        -Confirm:$false
    if ($LASTEXITCODE -ne 0) { throw "Migration entry failed with exit code $LASTEXITCODE." }
    $migrationResult = ($migrationOutput | Out-String | ConvertFrom-Json)

    Install-LocusArtifact -Source $artifact -LocusRoot $LocusRoot -Release $targetRelease
    $verified = (& $targetExecutable version --json | ConvertFrom-Json)
    if ($LASTEXITCODE -ne 0 -or $verified.version -ne $targetRelease.version -or $verified.artifact -ne 'complete') {
        throw 'Installed locus.exe failed the version smoke check.'
    }

    [ordered]@{
        status = 'deployed'
        from_version = $fromVersion
        to_version = $verified.version
        locus_root = $LocusRoot
        executable = $targetExecutable
        state_path = $StatePath
        state_migration = $migrationResult.status
        backup_dir = $backupDir
    } | ConvertTo-Json | Write-Output
}
catch {
    if ($null -ne $migrationResult -and $migrationResult.backup_path) {
        Copy-Item -LiteralPath $migrationResult.backup_path -Destination $StatePath -Force
    }
    elseif (-not $stateExisted -and (Test-Path -LiteralPath $StatePath)) {
        Remove-Item -LiteralPath $StatePath -Force
    }
    elseif ($emptyStateBackup) {
        Copy-Item -LiteralPath $emptyStateBackup -Destination $StatePath -Force
    }

    if ($hadInstallation) {
        New-Item -ItemType Directory -Force (Split-Path -Parent $targetExecutable) | Out-Null
        Copy-Item -LiteralPath (Join-Path $backupDir 'locus.exe') -Destination $targetExecutable -Force
        Copy-Item -LiteralPath (Join-Path $backupDir 'release.json') -Destination (Join-Path $LocusRoot 'release.json') -Force
    }
    else {
        Remove-Item -LiteralPath $targetExecutable -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath (Join-Path $LocusRoot 'release.json') -Force -ErrorAction SilentlyContinue
    }
    throw
}
