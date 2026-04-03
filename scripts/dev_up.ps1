param(
    [switch]$NoBrowser
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

$pathsToEnsure = @(
    (Join-Path $repoRoot "data"),
    (Join-Path $repoRoot "data\uploads"),
    (Join-Path $repoRoot "data\audio"),
    (Join-Path $repoRoot "data\screenshots"),
    (Join-Path $repoRoot "data\logs")
)

foreach ($path in $pathsToEnsure) {
    $null = New-Item -ItemType Directory -Force -Path $path
}

$webScript = Join-Path $repoRoot "scripts\dev_web.ps1"
$workerScript = Join-Path $repoRoot "scripts\dev_worker.ps1"

Start-Process powershell.exe -WorkingDirectory $repoRoot -ArgumentList @(
    "-NoExit",
    "-ExecutionPolicy", "Bypass",
    "-File", $webScript
) | Out-Null

Start-Sleep -Milliseconds 700

Start-Process powershell.exe -WorkingDirectory $repoRoot -ArgumentList @(
    "-NoExit",
    "-ExecutionPolicy", "Bypass",
    "-File", $workerScript
) | Out-Null

$webUrl = "http://localhost:8080/"
$healthUrl = "http://localhost:8080/health"
$webLog = Join-Path $repoRoot "data\logs\web.log"
$workerLog = Join-Path $repoRoot "data\logs\worker.log"

if (-not $NoBrowser) {
    Start-Process $webUrl | Out-Null
}

Write-Host ""
Write-Host "Local startup requested."
Write-Host "Web URL: $webUrl"
Write-Host "Health URL: $healthUrl"
Write-Host "Web logs: $webLog"
Write-Host "Worker logs: $workerLog"
Write-Host "Stop processes: close the two PowerShell windows or press Ctrl+C in each of them"
