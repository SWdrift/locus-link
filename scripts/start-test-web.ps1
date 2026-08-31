[CmdletBinding()]
param(
    [ValidatePattern('^127\.0\.0\.1:\d{1,5}$')]
    [string]$Address = '127.0.0.1:7070',
    [switch]$Refresh,
    [switch]$NoBrowser
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$runRoot = Join-Path $repo 'temp/e2e-run'
$locus = Join-Path $runRoot 'bin/locus.exe'
$project = Join-Path $runRoot 'projects/alpha'
$binding = Join-Path $runRoot 'mechanisms/workstation-a.yaml'
$state = Join-Path $runRoot 'state/state.db'
$device = Join-Path $runRoot 'devices/dev-a'
$probeLog = Join-Path $runRoot 'probe-invocations.log'

$required = @($locus, (Join-Path $project '.locus/registry/scope.yaml'), $binding, $state)
if ($Refresh -or ($required | Where-Object { -not (Test-Path -LiteralPath $_) })) {
    & (Join-Path $repo 'scripts/test-e2e.ps1')
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

$hostName, $portText = $Address -split ':', 2
$port = [int]$portText
$url = "http://$Address/graph"
$oldStatePath = $env:LOCUS_STATE_PATH
$oldSimRoot = $env:LOCUS_SIM_ROOT
$oldSimLog = $env:LOCUS_SIM_LOG
$oldPath = $env:PATH
$process = $null

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

    $ready = $false
    $deadline = [DateTime]::UtcNow.AddSeconds(15)
    while (-not $ready -and [DateTime]::UtcNow -lt $deadline) {
        if ($process.HasExited) {
            throw "locus web exited with code $($process.ExitCode)"
        }
        $client = [Net.Sockets.TcpClient]::new()
        try {
            $client.Connect($hostName, $port)
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
        throw "locus web did not listen at $Address within 15 seconds"
    }

    Write-Host "测试页面已启动：$url"
    Write-Host '按 Ctrl+C 停止服务。'
    if (-not $NoBrowser) {
        Start-Process $url
    }
    Wait-Process -Id $process.Id
}
finally {
    if ($null -ne $process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id
        $process.WaitForExit()
    }
    $env:LOCUS_STATE_PATH = $oldStatePath
    $env:LOCUS_SIM_ROOT = $oldSimRoot
    $env:LOCUS_SIM_LOG = $oldSimLog
    $env:PATH = $oldPath
}
