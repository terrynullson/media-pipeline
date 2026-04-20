param()

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot
. (Join-Path $PSScriptRoot "load_env.ps1") -RepoRoot $repoRoot

function Get-PostgresBinDir {
    $fromPath = Get-Command psql.exe -ErrorAction SilentlyContinue
    if ($fromPath) {
        return Split-Path -Parent $fromPath.Source
    }

    $candidates = Get-ChildItem "C:\Program Files\PostgreSQL" -Directory -ErrorAction SilentlyContinue |
        Sort-Object Name -Descending
    foreach ($candidate in $candidates) {
        $binDir = Join-Path $candidate.FullName "bin"
        if (Test-Path (Join-Path $binDir "pg_ctl.exe")) {
            return $binDir
        }
    }

    return $null
}

function Get-DescendantProcessIds {
    param([int]$RootId)

    $all = Get-CimInstance Win32_Process
    $result = New-Object System.Collections.Generic.List[int]

    function Add-Children {
        param(
            [int]$ParentId,
            [System.Collections.Generic.List[int]]$Buffer,
            $Processes
        )

        $children = $Processes | Where-Object { $_.ParentProcessId -eq $ParentId }
        foreach ($child in $children) {
            Add-Children -ParentId $child.ProcessId -Buffer $Buffer -Processes $Processes
            if (-not $Buffer.Contains([int]$child.ProcessId)) {
                $Buffer.Add([int]$child.ProcessId)
            }
        }
    }

    Add-Children -ParentId $RootId -Buffer $result -Processes $all
    return $result
}

function Stop-ProcessTree {
    param([int]$RootId)

    $ids = New-Object System.Collections.Generic.List[int]
    foreach ($id in (Get-DescendantProcessIds -RootId $RootId)) {
        if (-not $ids.Contains([int]$id)) {
            $ids.Add([int]$id)
        }
    }
    if (-not $ids.Contains($RootId)) {
        $ids.Add($RootId)
    }

    foreach ($id in $ids) {
        try {
            Stop-Process -Id $id -Force -ErrorAction Stop
        }
        catch {
        }
    }
}

function Stop-ProjectShell {
    param([string]$ScriptName)

    $escapedRoot = [Regex]::Escape($repoRoot)
    $escapedScript = [Regex]::Escape($ScriptName)
    $processes = Get-CimInstance Win32_Process | Where-Object {
        ($_.Name -ieq "powershell.exe" -or $_.Name -ieq "pwsh.exe") -and
        $_.CommandLine -match $escapedRoot -and
        $_.CommandLine -match $escapedScript
    }

    foreach ($process in $processes) {
        Stop-ProcessTree -RootId ([int]$process.ProcessId)
    }

    return @($processes).Count
}

$webStopped = Stop-ProjectShell -ScriptName "dev_web.ps1"
$workerStopped = Stop-ProjectShell -ScriptName "dev_worker.ps1"

$dataDir = Join-Path $repoRoot "data\postgres-local"
$binDir = Get-PostgresBinDir
$pgCtl = if ($binDir) { Join-Path $binDir "pg_ctl.exe" } else { $null }
$dbStopped = $false

if ($pgCtl -and (Test-Path $pgCtl) -and (Test-Path $dataDir)) {
    & $pgCtl -D $dataDir stop -m fast *> $null
    $dbStopped = $LASTEXITCODE -eq 0
}

Write-Host "Stopped web shells: $webStopped"
Write-Host "Stopped worker shells: $workerStopped"
if ($dbStopped) {
    Write-Host "Local PostgreSQL stopped."
} else {
    Write-Host "Local PostgreSQL was not running or could not be stopped automatically."
}
