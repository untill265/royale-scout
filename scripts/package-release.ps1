param(
    [Parameter(Mandatory = $false)]
    [ValidatePattern('^\d+\.\d+\.\d+([-.][0-9A-Za-z.-]+)?$')]
    [string]$Version = '0.1.0'
)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$releaseDirectory = Join-Path $projectRoot 'release'
$stageDirectory = Join-Path ([System.IO.Path]::GetTempPath()) "RoyaleScout-$Version-$([Guid]::NewGuid().ToString('N'))"
$temporaryRoot = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
$resolvedStageDirectory = [System.IO.Path]::GetFullPath($stageDirectory)
$archiveName = "RoyaleScout-v$Version-windows-x64.zip"
$archivePath = Join-Path $releaseDirectory $archiveName

if (-not $resolvedStageDirectory.StartsWith($temporaryRoot, [StringComparison]::OrdinalIgnoreCase) -or
    -not (Split-Path $resolvedStageDirectory -Leaf).StartsWith('RoyaleScout-', [StringComparison]::Ordinal)) {
    throw 'Refusing to use an unexpected staging directory.'
}

$wailsConfiguration = Get-Content -LiteralPath (Join-Path $projectRoot 'wails.json') -Raw | ConvertFrom-Json
if ($wailsConfiguration.info.productVersion -ne $Version) {
    throw "wails.json version '$($wailsConfiguration.info.productVersion)' does not match release version '$Version'."
}
$applicationSource = Get-Content -LiteralPath (Join-Path $projectRoot 'app.go') -Raw
$escapedVersion = [Regex]::Escape($Version)
if ($applicationSource -notmatch "applicationVersion\s*=\s*`"$escapedVersion`"") {
    throw "app.go applicationVersion does not match release version '$Version'."
}

try {
    Push-Location $projectRoot
    wails build -clean -trimpath -platform windows/amd64 -o RoyaleScout.exe

    New-Item -ItemType Directory -Path $stageDirectory | Out-Null
    New-Item -ItemType Directory -Path $releaseDirectory -Force | Out-Null

    $publicFiles = @(
        'README.md',
        'README.en.md',
        'PRIVACY.md',
        'SECURITY.md',
        'LICENSE',
        'THIRD_PARTY_NOTICES.md'
    )
    Copy-Item -LiteralPath (Join-Path $projectRoot 'build\bin\RoyaleScout.exe') -Destination $stageDirectory
    foreach ($file in $publicFiles) {
        Copy-Item -LiteralPath (Join-Path $projectRoot $file) -Destination $stageDirectory
    }
    & (Join-Path $PSScriptRoot 'generate-third-party-licenses.ps1') -OutputPath (Join-Path $stageDirectory 'THIRD_PARTY_LICENSES.txt')

    if (Test-Path -LiteralPath $archivePath) {
        Remove-Item -LiteralPath $archivePath -Force
    }
    Compress-Archive -Path (Join-Path $stageDirectory '*') -DestinationPath $archivePath -CompressionLevel Optimal

    $hash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    $checksumPath = Join-Path $releaseDirectory 'SHA256SUMS.txt'
    "$hash  $archiveName" | Set-Content -LiteralPath $checksumPath -Encoding ascii

    Write-Host "Created $archivePath"
    Write-Host "SHA-256 $hash"
}
finally {
    Pop-Location
    if (Test-Path -LiteralPath $resolvedStageDirectory) {
        Remove-Item -LiteralPath $resolvedStageDirectory -Recurse -Force
    }
}
