[CmdletBinding()]
param(
    [ValidatePattern('^127\.0\.0\.1:\d{1,5}$')]
    [string]$Address = '127.0.0.1:7070',
    [ValidatePattern('^127\.0\.0\.1:\d{1,5}$')]
    [string]$FrontendAddress = '127.0.0.1:5173',
    [switch]$Refresh,
    [switch]$Dev,
    [switch]$NoBrowser
)

$ErrorActionPreference = 'Stop'

function Wait-TcpEndpoint {
    param(
        [Diagnostics.Process]$TargetProcess,
        [string]$HostName,
        [int]$Port,
        [string]$Label
    )

    $ready = $false
    $deadline = [DateTime]::UtcNow.AddSeconds(15)
    while (-not $ready -and [DateTime]::UtcNow -lt $deadline) {
        if ($TargetProcess.HasExited) {
            throw "$Label exited with code $($TargetProcess.ExitCode)"
        }
        $client = [Net.Sockets.TcpClient]::new()
        try {
            $client.Connect($HostName, $Port)
            $ready = $true
        }
        catch {
            Start-Sleep -Milliseconds 150
        }
        finally {
            $client.Dispose()
        }
    }
    if (-not $ready) {
        throw "$Label did not listen at ${HostName}:$Port within 15 seconds"
    }
}
$repo = Split-Path -Parent $PSScriptRoot
$runRoot = Join-Path $repo 'temp/e2e-run/native'
$locus = Join-Path $runRoot 'bin/locus.exe'
$project = Join-Path $runRoot 'projects/alpha'
$binding = Join-Path $runRoot 'mechanisms/workstation-a.yaml'
$state = Join-Path $runRoot 'state/state.db'
$device = Join-Path $runRoot 'devices/dev-a'
$probeLog = Join-Path $runRoot 'probe-invocations.log'

$required = @($locus, (Join-Path $project '.locus/registry/scope.yaml'), $binding, $state)
$missing = @($required | Where-Object { -not (Test-Path -LiteralPath $_) })
if ($Refresh -or $missing.Count -gt 0) {
    & (Join-Path $repo 'scripts/test-e2e.ps1')
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
$missing = @($required | Where-Object { -not (Test-Path -LiteralPath $_) })
if ($missing.Count -gt 0) {
    throw "E2E artifacts are missing after refresh: $($missing -join ', ')"
}

$hostName, $portText = $Address -split ':', 2
$port = [int]$portText
$frontendHostName, $frontendPortText = $FrontendAddress -split ':', 2
$frontendPort = [int]$frontendPortText
$url = "http://$Address/graph"
$oldStatePath = $env:LOCUS_STATE_PATH
$oldSimRoot = $env:LOCUS_SIM_ROOT
$oldSimLog = $env:LOCUS_SIM_LOG
$oldPath = $env:PATH
$oldApiOrigin = $env:LOCUS_API_ORIGIN
$process = $null
$frontendProcess = $null

try {
    $env:LOCUS_STATE_PATH = $state
    $env:LOCUS_SIM_ROOT = $device
    $env:LOCUS_SIM_LOG = $probeLog
    $env:PATH = "$(Join-Path $runRoot 'bin');$oldPath"

    $arguments = @(
        'web',
        '--from', 'workstation.dev-a',
        '--vantage', 'office-lan',
        '--mechanism-bindings', "`"$binding`"",
        '--address', $Address
    )
    $process = Start-Process -FilePath $locus -ArgumentList $arguments -WorkingDirectory $project -NoNewWindow -PassThru

    Wait-TcpEndpoint -TargetProcess $process -HostName $hostName -Port $port -Label 'locus web'

    if ($Dev) {
        $ui = Join-Path $repo 'internal/web/ui'
        $vite = Join-Path $ui 'node_modules/vite/bin/vite.js'
        $nodeCommand = Get-Command node -CommandType Application -ErrorAction SilentlyContinue
        if ($null -eq $nodeCommand) {
            throw 'Node.js executable was not found on PATH.'
        }
        if (-not (Test-Path -LiteralPath $vite)) {
            throw "Vite was not found at $vite. Run pnpm install in internal/web/ui first."
        }

        $env:LOCUS_API_ORIGIN = "http://$Address"
        $viteArguments = @(
            "`"$vite`"",
            '--host', $frontendHostName,
            '--port', $frontendPort
        )
        $frontendProcess = Start-Process -FilePath $nodeCommand.Source -ArgumentList $viteArguments -WorkingDirectory $ui -NoNewWindow -PassThru
        Wait-TcpEndpoint -TargetProcess $frontendProcess -HostName $frontendHostName -Port $frontendPort -Label 'Vite'
        $url = "http://$FrontendAddress/graph"
    }

    Write-Host "Web debug page started: $url"
    Write-Host 'Press Ctrl+C to stop both servers.'
    if (-not $NoBrowser) {
        Start-Process $url
    }
    while (-not $process.HasExited -and ($null -eq $frontendProcess -or -not $frontendProcess.HasExited)) {
        Start-Sleep -Milliseconds 250
    }
    if ($process.HasExited) {
        throw "locus web exited with code $($process.ExitCode)"
    }
    throw "Vite exited with code $($frontendProcess.ExitCode)"
}
finally {
    if ($null -ne $frontendProcess -and -not $frontendProcess.HasExited) {
        Stop-Process -Id $frontendProcess.Id
        $frontendProcess.WaitForExit()
    }
    if ($null -ne $process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id
        $process.WaitForExit()
    }
    $env:LOCUS_STATE_PATH = $oldStatePath
    $env:LOCUS_SIM_ROOT = $oldSimRoot
    $env:LOCUS_SIM_LOG = $oldSimLog
    $env:LOCUS_API_ORIGIN = $oldApiOrigin
    $env:PATH = $oldPath
}
