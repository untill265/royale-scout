param(
    [Parameter(Mandatory = $true)]
    [string]$OutputPath
)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$builder = [System.Text.StringBuilder]::new()

[void]$builder.AppendLine('Royale Scout — third-party licenses')
[void]$builder.AppendLine('Generated from dependency lock files during release packaging.')
[void]$builder.AppendLine()
[void]$builder.AppendLine('Supercell game names and API-delivered artwork are not covered by these licenses.')
[void]$builder.AppendLine('See THIRD_PARTY_NOTICES.md and https://supercell.com/en/fan-content-policy/.')

function Add-License {
    param(
        [string]$Name,
        [string]$Version,
        [string]$Directory
    )

    if ([string]::IsNullOrWhiteSpace($Directory) -or -not (Test-Path -LiteralPath $Directory -PathType Container)) {
        return
    }
    $license = Get-ChildItem -LiteralPath $Directory -File | Where-Object {
        $_.Name -match '^(LICENSE|LICENCE|COPYING|NOTICE)(\..*)?$'
    } | Sort-Object Name | Select-Object -First 1
    if (-not $license) {
        return
    }

    [void]$builder.AppendLine()
    [void]$builder.AppendLine(('=' * 78))
    [void]$builder.AppendLine("$Name $Version")
    [void]$builder.AppendLine(('=' * 78))
    [void]$builder.AppendLine([System.IO.File]::ReadAllText($license.FullName))
}

Push-Location $projectRoot
try {
    $goModules = & go list -m -f '{{.Path}}|{{.Version}}|{{.Dir}}' all
    foreach ($module in $goModules) {
        $parts = $module -split '\|', 3
        if ($parts.Count -eq 3 -and $parts[0] -ne 'royalescout') {
            Add-License -Name $parts[0] -Version $parts[1] -Directory $parts[2]
        }
    }

    $nodeDirectories = & npm ls --prefix frontend --omit=dev --parseable 2>$null
    foreach ($directory in $nodeDirectories) {
        if ($directory -eq (Resolve-Path 'frontend').Path) {
            continue
        }
        $packageFile = Join-Path $directory 'package.json'
        if (Test-Path -LiteralPath $packageFile) {
            $package = Get-Content -LiteralPath $packageFile -Raw | ConvertFrom-Json
            Add-License -Name $package.name -Version $package.version -Directory $directory
        }
    }
}
finally {
    Pop-Location
}

$resolvedOutput = [System.IO.Path]::GetFullPath($OutputPath)
$parent = Split-Path -Parent $resolvedOutput
New-Item -ItemType Directory -Path $parent -Force | Out-Null
[System.IO.File]::WriteAllText($resolvedOutput, $builder.ToString(), [System.Text.UTF8Encoding]::new($false))
