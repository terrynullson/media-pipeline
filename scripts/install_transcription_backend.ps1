param(
    [switch]$ForceReinstall
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

function Test-PythonCommand {
    param([string]$CommandName, [string[]]$Arguments)

    $command = Get-Command $CommandName -ErrorAction SilentlyContinue
    if (-not $command) {
        return $false
    }

    & $CommandName @Arguments *> $null
    return $LASTEXITCODE -eq 0
}

function Resolve-BootstrapPython {
    if (Test-PythonCommand -CommandName "py" -Arguments @("-3", "--version")) {
        return @{
            Command = "py"
            Args = @("-3")
            Label = "py -3"
        }
    }
    if (Test-PythonCommand -CommandName "python" -Arguments @("--version")) {
        return @{
            Command = "python"
            Args = @()
            Label = "python"
        }
    }

    throw "Python launcher was not found. Install Python 3 and make sure either 'py' or 'python' works in PowerShell."
}

$venvPath = Join-Path $repoRoot ".venv"
$venvPython = Join-Path $venvPath "Scripts\python.exe"
$requirementsPath = Join-Path $repoRoot "scripts\requirements-transcription.txt"

$bootstrap = Resolve-BootstrapPython

Write-Host "Repository: $repoRoot"
Write-Host "Python bootstrap command: $($bootstrap.Label)"

if ($ForceReinstall -and (Test-Path $venvPath)) {
    Write-Host "Removing existing virtual environment: $venvPath"
    Remove-Item -Recurse -Force -LiteralPath $venvPath
}

if (-not (Test-Path $venvPython)) {
    Write-Host "Creating virtual environment in $venvPath"
    & $bootstrap.Command @($bootstrap.Args + @("-m", "venv", $venvPath))
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to create virtual environment."
    }
}

Write-Host "Using virtual environment Python: $venvPython"

& $venvPython -m pip install --upgrade pip
if ($LASTEXITCODE -ne 0) {
    throw "Failed to upgrade pip."
}

& $venvPython -m pip install -r $requirementsPath
if ($LASTEXITCODE -ne 0) {
    throw "Failed to install transcription requirements."
}

Write-Host "Running backend self-check..."
& $venvPython .\scripts\transcribe.py --self-check
if ($LASTEXITCODE -ne 0) {
    throw "Transcription backend self-check failed."
}

Write-Host ""
Write-Host "Transcription backend is installed."
Write-Host "Worker will use: $venvPython"
Write-Host "If you start manually, set PYTHON_BINARY=$venvPython"
Write-Host "Default model for local CPU runs is WHISPER_MODEL=tiny"
