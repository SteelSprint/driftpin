$ErrorActionPreference = "Stop"
# #F id:te2ztzzz install.scripts install.platform_detection install.version install.checksum install.archive_validation install.no_overwrite install.permissions install.no_temp_cleanup install.install_dir install.path_check install.network_security install.errors

[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

Add-Type -AssemblyName System.IO.Compression.FileSystem

$REPO = "steelsprint/filament"
$BINARY = "filament"

function Get-Arch {
    $arch = [Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE", "Machine")
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default {
            throw "Unsupported architecture: $arch. Download manually from https://github.com/$REPO/releases."
        }
    }
}

function Get-LatestVersion {
    $url = "https://api.github.com/repos/$REPO/releases/latest"
    $response = Invoke-RestMethod -Uri $url -UseBasicParsing
    if (-not $response.tag_name) {
        throw "Could not determine latest version from GitHub API."
    }
    return $response.tag_name
}

function Verify-Checksum {
    param (
        [string]$FilePath,
        [string]$ChecksumsPath,
        [string]$Filename
    )

    $expected = $null
    foreach ($line in Get-Content $ChecksumsPath) {
        $parts = $line -split '\s+'
        if ($parts.Count -ge 2 -and $parts[1] -ieq $Filename) {
            $expected = $parts[0]
            break
        }
    }

    if (-not $expected) {
        throw "No checksum found for $Filename in checksums file."
    }

    $actual = (Get-FileHash -Path $FilePath -Algorithm SHA256).Hash.ToLower()
    if ($actual -ne $expected.ToLower()) {
        throw "Checksum mismatch for $Filename`n  expected: $expected`n  actual:   $actual"
    }
}

function Main {
    $arch = Get-Arch
    $version = Get-LatestVersion

    $archive = "${BINARY}_windows_${arch}.zip"
    $baseUrl = "https://github.com/$REPO/releases/download/$version"

    $installDir = "$env:USERPROFILE\.filament\bin"

    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "filament-install-$([System.Guid]::NewGuid().ToString('N').Substring(0,8))"
    New-Item -ItemType Directory -Path $tmpDir | Out-Null

    Write-Host "Downloading $BINARY $version for windows_$arch..."
    $archivePath = Join-Path $tmpDir $archive
    $checksumsPath = Join-Path $tmpDir "checksums.txt"

    Invoke-WebRequest -Uri "$baseUrl/$archive" -OutFile $archivePath -UseBasicParsing
    Invoke-WebRequest -Uri "$baseUrl/checksums.txt" -OutFile $checksumsPath -UseBasicParsing

    Write-Host "Verifying checksum..."
    Verify-Checksum -FilePath $archivePath -ChecksumsPath $checksumsPath -Filename $archive

    Write-Host "Validating archive..."
    $zip = [System.IO.Compression.ZipFile]::OpenRead($archivePath)
    try {
        $entries = @($zip.Entries)
        if ($entries.Count -ne 1) {
            throw "Archive contains $($entries.Count) entries, expected exactly 1 ($BINARY.exe). Entries: $($entries.FullName -join ', ')"
        }
        if ($entries[0].FullName -ne "$BINARY.exe") {
            throw "Archive contains '$($entries[0].FullName)', expected '$BINARY.exe'."
        }
    } finally {
        $zip.Dispose()
    }

    Write-Host "Extracting..."
    Expand-Archive -Path $archivePath -DestinationPath $tmpDir -Force

    if (-not (Test-Path $installDir)) {
        New-Item -ItemType Directory -Path $installDir | Out-Null
    }

    $binarySrc = Join-Path $tmpDir "$BINARY.exe"
    $binaryDst = Join-Path $installDir "$BINARY.exe"

    if (Test-Path $binaryDst) {
        throw "$BINARY.exe already exists at $binaryDst. Remove it first:`n  Remove-Item `"$binaryDst`""
    }

    Move-Item -Path $binarySrc -Destination $binaryDst -Force

    Write-Host "Installed $BINARY $version to $binaryDst"

    $pathEntries = $env:Path -split ';'
    $inPath = $false
    foreach ($entry in $pathEntries) {
        if ($entry -ieq $installDir) {
            $inPath = $true
            break
        }
    }

    if (-not $inPath) {
        Write-Host ""
        Write-Host "NOTE: $installDir is not in your PATH."
        Write-Host "Add it with:"
        Write-Host ""
        Write-Host "  setx PATH `"%PATH%;$installDir`""
        Write-Host ""
        Write-Host "Or run: `$env:Path += ';$installDir'"
    }
}

Main
