param([switch]$Remove, [switch]$FromSource)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\agentic"
$InstallPath = Join-Path $InstallDir "agentic.exe"
$DataDir = if ($env:AGENTIC_HOME) { $env:AGENTIC_HOME } else { Join-Path $env:APPDATA "agentic" }

function Install-FromSource {
  if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    Write-Error "Docker is not installed or not on PATH."
    exit 1
  }

  if ($Arch -eq "arm64") {
    Write-Error "Windows arm64 is not supported for source builds via Docker. Only amd64 is supported."
    exit 1
  }

  $BinaryName = "agentic-windows-$Arch.exe"
  $BinarySrc = Join-Path $ScriptDir "dist\$BinaryName"

  Write-Host "Building agentic for windows/$Arch..."
  & docker buildx build `
    --target export `
    --build-arg "INSTALL_METHOD=script-pwsh" `
    --output "$ScriptDir\dist\" `
    $ScriptDir
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

  if (-not (Test-Path $BinarySrc)) {
    Write-Error "Expected binary not found at $BinarySrc"
    exit 1
  }

  New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
  Copy-Item $BinarySrc $InstallPath -Force
  Write-Host "Installed agentic to $InstallPath"
}

function Install-FromRelease {
  if ($Arch -eq "arm64") {
    Write-Error "Windows arm64 is not supported. Use -FromSource to build from source via Docker."
    exit 1
  }

  Write-Host "Fetching latest release..."
  try {
    $Release = Invoke-RestMethod "https://api.github.com/repos/dylanvgils/agentic-cli/releases/latest"
    $Version = $Release.tag_name -replace '^v', ''
  } catch {
    Write-Error "Failed to fetch latest release: $_"
    exit 1
  }

  $Archive = "agentic-$Version-windows-amd64.zip"
  $Url = "https://github.com/dylanvgils/agentic-cli/releases/download/v$Version/$Archive"
  $ChecksumsUrl = "https://github.com/dylanvgils/agentic-cli/releases/download/v$Version/checksums.txt"

  $TmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid())
  New-Item -ItemType Directory -Force -Path $TmpDir | Out-Null
  try {
    Write-Host "Downloading agentic $Version for windows/amd64..."
    Invoke-WebRequest -Uri $Url -OutFile (Join-Path $TmpDir $Archive) -UseBasicParsing
    Invoke-WebRequest -Uri $ChecksumsUrl -OutFile (Join-Path $TmpDir "checksums.txt") -UseBasicParsing

    Write-Host "Verifying checksum..."
    $ChecksumLine = Get-Content (Join-Path $TmpDir "checksums.txt") | Where-Object { $_ -match " $([regex]::Escape($Archive))$" }
    if (-not $ChecksumLine) {
      Write-Error "Checksum not found for $Archive"
      exit 1
    }
    $Expected = ($ChecksumLine -split '\s+')[0]
    $Actual = (Get-FileHash (Join-Path $TmpDir $Archive) -Algorithm SHA256).Hash.ToLower()
    if ($Actual -ne $Expected) {
      Write-Error "Checksum mismatch for $Archive"
      exit 1
    }

    Expand-Archive -Path (Join-Path $TmpDir $Archive) -DestinationPath $TmpDir -Force

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item (Join-Path $TmpDir "agentic.exe") $InstallPath -Force
    Write-Host "Installed agentic to $InstallPath"
  } finally {
    Remove-Item -Force -Recurse $TmpDir -ErrorAction SilentlyContinue
  }
}

$Arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }

if ($Remove) {
  if (Test-Path $InstallPath) {
    Remove-Item -Force $InstallPath
    Write-Host "Removed $InstallPath"
  }

  if ((Test-Path $InstallDir) -and -not (Get-ChildItem $InstallDir)) {
    Remove-Item -Force -Recurse $InstallDir
  }

  if (Test-Path $DataDir) {
    $confirm = Read-Host "Remove data directory $DataDir? [y/N]"
    if ($confirm -match '^[Yy]$') {
      Remove-Item -Force -Recurse $DataDir
      Write-Host "Removed $DataDir"
    }
  }

  $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
  if ($userPath -like "*$InstallDir*") {
    $newPath = ($userPath -split ";" | Where-Object { $_ -ne $InstallDir }) -join ";"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "Removed $InstallDir from user PATH"
  }

  exit 0
}

if ($FromSource) {
  Install-FromSource
} else {
  Install-FromRelease
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
  [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
  Write-Host "Added $InstallDir to user PATH"
  Write-Host "Note: restart your terminal for the PATH change to take effect"
}
