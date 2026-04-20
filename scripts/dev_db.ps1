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

    throw "PostgreSQL binaries were not found. Install PostgreSQL or add psql/pg_ctl to PATH."
}

function Get-DsnParts {
    param([Parameter(Mandatory = $true)][string]$Dsn)

    $uri = [Uri]$Dsn
    $userInfo = $uri.UserInfo.Split(":", 2)

    return [pscustomobject]@{
        Host     = $uri.Host
        Port     = $uri.Port
        User     = $userInfo[0]
        Password = if ($userInfo.Count -gt 1) { $userInfo[1] } else { "" }
        Database = $uri.AbsolutePath.TrimStart("/")
    }
}

function Ensure-DatabaseExists {
    param(
        [string]$BinDir,
        [pscustomobject]$Parts,
        [string]$DatabaseName
    )

    $env:PGPASSWORD = $Parts.Password
    $exists = & (Join-Path $BinDir "psql.exe") `
        -h $Parts.Host `
        -p $Parts.Port `
        -U $Parts.User `
        -d postgres `
        -tAc "SELECT 1 FROM pg_database WHERE datname = '$DatabaseName'"

    if ($LASTEXITCODE -ne 0) {
        throw "Failed to check PostgreSQL database existence for $DatabaseName."
    }

    $existsValue = ""
    if ($null -ne $exists) {
        $existsValue = ($exists | Out-String).Trim()
    }

    if ($existsValue -eq "1") {
        return
    }

    & (Join-Path $BinDir "createdb.exe") -h $Parts.Host -p $Parts.Port -U $Parts.User $DatabaseName
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to create PostgreSQL database $DatabaseName."
    }
}

if (-not $env:DATABASE_URL) {
    throw "DATABASE_URL is not configured in .env."
}

$parts = Get-DsnParts -Dsn $env:DATABASE_URL
$testParts = if ($env:TEST_DATABASE_URL) { Get-DsnParts -Dsn $env:TEST_DATABASE_URL } else { $null }
$binDir = Get-PostgresBinDir
$dataDir = Join-Path $repoRoot "data\postgres-local"
$passwordFile = Join-Path $repoRoot "data\postgres-local.pw"
$pgCtl = Join-Path $binDir "pg_ctl.exe"
$pgIsReady = Join-Path $binDir "pg_isready.exe"

$isReady = & $pgIsReady -h $parts.Host -p $parts.Port
if ($LASTEXITCODE -ne 0) {
    if (-not (Test-Path $dataDir)) {
        $null = New-Item -ItemType Directory -Force -Path (Split-Path -Parent $passwordFile)
        Set-Content -NoNewline -Path $passwordFile -Value $parts.Password
        & (Join-Path $binDir "initdb.exe") `
            -D $dataDir `
            -U $parts.User `
            -A scram-sha-256 `
            --encoding=UTF8 `
            --locale=C `
            --pwfile=$passwordFile
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to initialize local PostgreSQL cluster in $dataDir."
        }
    }

    $logFile = Join-Path $repoRoot "data\logs\postgres-local.log"
    & $pgCtl -D $dataDir -l $logFile -o " -p $($parts.Port)" start
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to start local PostgreSQL on port $($parts.Port)."
    }
}

$ready = $false
for ($attempt = 0; $attempt -lt 30; $attempt++) {
    & $pgIsReady -h $parts.Host -p $parts.Port *> $null
    if ($LASTEXITCODE -eq 0) {
        $ready = $true
        break
    }
    Start-Sleep -Seconds 1
}

if (-not $ready) {
    throw "Local PostgreSQL did not become ready on $($parts.Host):$($parts.Port)."
}

Ensure-DatabaseExists -BinDir $binDir -Parts $parts -DatabaseName $parts.Database
if ($testParts) {
    Ensure-DatabaseExists -BinDir $binDir -Parts $parts -DatabaseName $testParts.Database
}

Write-Host "Local PostgreSQL is ready at $($parts.Host):$($parts.Port)"
Write-Host "Main DB: $($parts.Database)"
if ($testParts) {
    Write-Host "Test DB: $($testParts.Database)"
}
