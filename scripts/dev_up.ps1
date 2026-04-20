param(
    [switch]$NoBrowser
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot
. (Join-Path $PSScriptRoot "load_env.ps1") -RepoRoot $repoRoot

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
$dbScript = Join-Path $repoRoot "scripts\dev_db.ps1"

& $dbScript

Start-Process powershell.exe -WorkingDirectory $repoRoot -ArgumentList @(
    "-NoExit",
    "-ExecutionPolicy", "Bypass",
    "-File", $webScript
) | Out-Null

$appPort = if ($env:APP_PORT) { $env:APP_PORT } else { "8080" }
$healthUrl = "http://localhost:$appPort/health"
$ready = $false
for ($attempt = 0; $attempt -lt 60; $attempt++) {
    Start-Sleep -Seconds 1

    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $healthUrl -TimeoutSec 2
        if ($response.StatusCode -eq 200) {
            $ready = $true
            break
        }
    }
    catch {
    }
}

if (-not $ready) {
    throw "Web service did not become healthy at $healthUrl within 60 seconds."
}

Start-Process powershell.exe -WorkingDirectory $repoRoot -ArgumentList @(
    "-NoExit",
    "-ExecutionPolicy", "Bypass",
    "-File", $workerScript
) | Out-Null

$webUrl = "http://localhost:$appPort/app-v1"
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
Write-Host "Stop command: .\\stop.bat"
