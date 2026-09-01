function Get-LocusReleaseMetadata {
    param([Parameter(Mandatory = $true)][string]$LocusRoot)

    $executable = Join-Path $LocusRoot 'bin/locus.exe'
    $manifest = Join-Path $LocusRoot 'release.json'
    $hasExecutable = Test-Path -LiteralPath $executable -PathType Leaf
    $hasManifest = Test-Path -LiteralPath $manifest -PathType Leaf
    if (-not $hasExecutable -and -not $hasManifest) { return $null }
    if (-not $hasExecutable -or -not $hasManifest) {
        throw "Installation is incomplete. Expected both $executable and $manifest."
    }
    $release = Get-Content -LiteralPath $manifest -Raw | ConvertFrom-Json
    if ([string]::IsNullOrWhiteSpace($release.version)) {
        throw "Installation manifest has no version: $manifest"
    }
    return $release
}

function Assert-LocusUpgradePath {
    param(
        [AllowNull()][object]$Installed,
        [Parameter(Mandatory = $true)][object]$Target
    )

    if ($null -eq $Installed) { return }
    if ($Installed.version -eq $Target.version) { return }
    if ($Installed.version -eq $Target.previous_version) { return }
    throw "Unsupported upgrade $($Installed.version) -> $($Target.version). Only $($Target.previous_version) -> $($Target.version) is supported."
}

function Assert-LocusExecutableStopped {
    param([Parameter(Mandatory = $true)][string]$Executable)

    if (-not (Test-Path -LiteralPath $Executable -PathType Leaf)) { return }
    $target = [System.IO.Path]::GetFullPath($Executable)
    $running = Get-Process | Where-Object {
        try { $_.Path -and [System.IO.Path]::GetFullPath($_.Path) -eq $target }
        catch { $false }
    }
    if ($running) {
        $ids = ($running | ForEach-Object Id) -join ', '
        throw "locus.exe is running from the installation directory (PID: $ids). Stop it before deployment."
    }
}

function Move-LocusFile {
    param(
        [Parameter(Mandatory = $true)][string]$Source,
        [Parameter(Mandatory = $true)][string]$Destination
    )

    if (Test-Path -LiteralPath $Destination -PathType Leaf) {
        $replacementBackup = $Destination + '.replace-' + [guid]::NewGuid().ToString('N')
        try {
            [System.IO.File]::Replace($Source, $Destination, $replacementBackup, $true)
        }
        finally {
            if (Test-Path -LiteralPath $replacementBackup) { Remove-Item -LiteralPath $replacementBackup -Force }
        }
    }
    else {
        [System.IO.File]::Move($Source, $Destination)
    }
}

function Get-LocusSha256 {
    param([Parameter(Mandatory = $true)][string]$Path)

    $stream = [System.IO.File]::OpenRead($Path)
    $sha256 = [System.Security.Cryptography.SHA256]::Create()
    try {
        return ([System.BitConverter]::ToString($sha256.ComputeHash($stream))).Replace('-', '').ToLowerInvariant()
    }
    finally {
        $sha256.Dispose()
        $stream.Dispose()
    }
}

function Install-LocusArtifact {
    param(
        [Parameter(Mandatory = $true)][string]$Source,
        [Parameter(Mandatory = $true)][string]$LocusRoot,
        [Parameter(Mandatory = $true)][object]$Release
    )

    $binDir = Join-Path $LocusRoot 'bin'
    New-Item -ItemType Directory -Force $binDir | Out-Null
    $target = Join-Path $binDir 'locus.exe'
    $staged = Join-Path $binDir ('.locus.exe.new-' + [guid]::NewGuid().ToString('N'))
    $manifestTemp = Join-Path $LocusRoot ('.release.json.new-' + [guid]::NewGuid().ToString('N'))
    try {
        Copy-Item -LiteralPath $Source -Destination $staged
        Move-LocusFile -Source $staged -Destination $target

        $manifest = [ordered]@{
            version = $Release.version
            previous_version = $Release.previous_version
            state_schema_version = $Release.state_schema_version
            commit = $Release.commit
            artifact = $Release.artifact
            sha256 = Get-LocusSha256 -Path $target
            installed_at = [DateTime]::UtcNow.ToString('o')
        }
        $manifestPath = Join-Path $LocusRoot 'release.json'
        $manifest | ConvertTo-Json | Set-Content -LiteralPath $manifestTemp -Encoding UTF8
        Move-LocusFile -Source $manifestTemp -Destination $manifestPath
    }
    finally {
        if (Test-Path -LiteralPath $staged) { Remove-Item -LiteralPath $staged -Force }
        if (Test-Path -LiteralPath $manifestTemp) { Remove-Item -LiteralPath $manifestTemp -Force }
    }
}
