param()

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot
. (Join-Path $PSScriptRoot "load_env.ps1") -RepoRoot $repoRoot

function Use-Utf8Console {
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [Console]::InputEncoding = $utf8NoBom
    [Console]::OutputEncoding = $utf8NoBom
    $script:OutputEncoding = $utf8NoBom

    try {
        chcp.com 65001 > $null
    }
    catch {
    }
}

Use-Utf8Console

$logDir = Join-Path $repoRoot "data\logs"
$null = New-Item -ItemType Directory -Force -Path $logDir

function Test-PythonCommand {
    param([string]$CommandName, [string[]]$Arguments)

    $command = Get-Command $CommandName -ErrorAction SilentlyContinue
    if (-not $command) {
        return $false
    }

    & $CommandName @Arguments *> $null
    return $LASTEXITCODE -eq 0
}

function Resolve-PythonBinary {
    $venvPython = Join-Path $repoRoot ".venv\Scripts\python.exe"
    if (Test-Path $venvPython) {
        return $venvPython
    }
    if (Test-PythonCommand -CommandName "py" -Arguments @("-3", "--version")) {
        return "py"
    }
    if (Test-PythonCommand -CommandName "python" -Arguments @("--version")) {
        return "python"
    }

    return "python"
}

function Test-DatabaseConfigPresent {
    if ($env:DATABASE_URL) {
        return $true
    }

    return [bool]($env:DB_HOST -and $env:DB_NAME -and $env:DB_USER)
}

$env:APP_PORT = if ($env:APP_PORT) { $env:APP_PORT } else { "8080" }
$env:UPLOAD_DIR = if ($env:UPLOAD_DIR) { $env:UPLOAD_DIR } else { ".\data\uploads" }
$env:AUDIO_DIR = if ($env:AUDIO_DIR) { $env:AUDIO_DIR } else { ".\data\audio" }
$env:TRANSCRIBE_SCRIPT = if ($env:TRANSCRIBE_SCRIPT) { $env:TRANSCRIBE_SCRIPT } else { ".\scripts\transcribe.py" }
$env:PYTHON_BINARY = if ($env:PYTHON_BINARY) { $env:PYTHON_BINARY } else { Resolve-PythonBinary }

if (-not (Test-DatabaseConfigPresent)) {
    Write-Error "Database is not configured. Set DATABASE_URL or DB_HOST/DB_NAME/DB_USER before starting web."
    exit 1
}

$logPath = Join-Path $logDir "web.log"

Write-Host "Starting web app..."
Write-Host "App URL: http://localhost:$($env:APP_PORT)/app-v1"
Write-Host "Health: http://localhost:$($env:APP_PORT)/health"
Write-Host "Database: PostgreSQL (configured via DATABASE_URL / DB_*)"
Write-Host "Log file: $logPath"
Write-Host "Stop: press Ctrl+C in this window"

go run ./cmd/web
