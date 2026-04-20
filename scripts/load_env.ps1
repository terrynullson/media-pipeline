param(
    [string]$RepoRoot
)

if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    throw "RepoRoot is required"
}

$envPath = Join-Path $RepoRoot ".env"
if (-not (Test-Path $envPath)) {
    return
}

Get-Content $envPath | ForEach-Object {
    $line = $_.Trim()
    if ([string]::IsNullOrWhiteSpace($line)) {
        return
    }
    if ($line.StartsWith("#")) {
        return
    }

    $separatorIndex = $line.IndexOf("=")
    if ($separatorIndex -lt 1) {
        return
    }

    $name = $line.Substring(0, $separatorIndex).Trim()
    $value = $line.Substring($separatorIndex + 1).Trim()

    if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
        $value = $value.Substring(1, $value.Length - 2)
    }

    if (-not [string]::IsNullOrWhiteSpace($name)) {
        [Environment]::SetEnvironmentVariable($name, $value, "Process")
    }
}
