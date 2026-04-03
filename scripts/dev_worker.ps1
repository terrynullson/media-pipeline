param()

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

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

$env:APP_PORT = if ($env:APP_PORT) { $env:APP_PORT } else { "8080" }
$env:DB_PATH = if ($env:DB_PATH) { $env:DB_PATH } else { ".\data\app.db" }
$env:UPLOAD_DIR = if ($env:UPLOAD_DIR) { $env:UPLOAD_DIR } else { ".\data\uploads" }
$env:AUDIO_DIR = if ($env:AUDIO_DIR) { $env:AUDIO_DIR } else { ".\data\audio" }
$env:TRANSCRIBE_SCRIPT = if ($env:TRANSCRIBE_SCRIPT) { $env:TRANSCRIBE_SCRIPT } else { ".\scripts\transcribe.py" }
$env:PYTHON_BINARY = if ($env:PYTHON_BINARY) { $env:PYTHON_BINARY } else { Resolve-PythonBinary }
$env:WHISPER_MODEL = if ($env:WHISPER_MODEL) { $env:WHISPER_MODEL } else { "tiny" }
$env:WHISPER_DEVICE = if ($env:WHISPER_DEVICE) { $env:WHISPER_DEVICE } else { "cpu" }
$env:WHISPER_COMPUTE_TYPE = if ($env:WHISPER_COMPUTE_TYPE) { $env:WHISPER_COMPUTE_TYPE } else { "int8" }

$logPath = Join-Path $logDir "worker.log"

Write-Host "Starting worker..."
Write-Host "Web URL: http://localhost:$($env:APP_PORT)/"
Write-Host "Health: http://localhost:$($env:APP_PORT)/health"
Write-Host "Python binary: $($env:PYTHON_BINARY)"
Write-Host "Whisper model: $($env:WHISPER_MODEL)"
Write-Host "Log file: $logPath"
Write-Host "Stop: press Ctrl+C in this window"

go run ./cmd/worker
