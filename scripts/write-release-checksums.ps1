param(
    [Parameter(Mandatory = $false)]
    [string]$ReleaseDirectory,

    [Parameter(Mandatory = $false)]
    [string[]]$FileName
)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($ReleaseDirectory)) {
    $ReleaseDirectory = Join-Path $projectRoot 'release'
}

$resolvedReleaseDirectory = [System.IO.Path]::GetFullPath($ReleaseDirectory)
if (-not (Test-Path -LiteralPath $resolvedReleaseDirectory -PathType Container)) {
    throw "Release directory does not exist: $resolvedReleaseDirectory"
}

if ($FileName) {
    $files = foreach ($name in $FileName) {
        if ($name -ne [System.IO.Path]::GetFileName($name) -or $name -eq 'SHA256SUMS.txt') {
            throw "Invalid release file name: $name"
        }
        $path = Join-Path $resolvedReleaseDirectory $name
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
            throw "Release file does not exist: $path"
        }
        Get-Item -LiteralPath $path
    }
    $files = $files | Sort-Object Name -Unique
}
else {
    $files = Get-ChildItem -LiteralPath $resolvedReleaseDirectory -File |
        Where-Object { $_.Name -ne 'SHA256SUMS.txt' } |
        Sort-Object Name
}

if (-not $files) {
    throw 'No release files were found for checksum generation.'
}

$lines = foreach ($file in $files) {
    $hash = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  $($file.Name)"
}

$checksumPath = Join-Path $resolvedReleaseDirectory 'SHA256SUMS.txt'
$lines | Set-Content -LiteralPath $checksumPath -Encoding ascii
$lines | ForEach-Object { Write-Host $_ }
