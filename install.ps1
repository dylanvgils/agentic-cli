param([switch]$Remove)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\agentic"
$InstallPath = Join-Path $InstallDir "agentic.exe"
$DataDir = if ($env:AGENTIC_HOME) { $env:AGENTIC_HOME } else { Join-Path $env:APPDATA "agentic" }

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

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
  Write-Error "Docker is not installed or not on PATH."
  exit 1
}

$Arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
$BinaryName = "agentic-windows-$Arch.exe"
$BinarySrc = Join-Path $ScriptDir "dist\$BinaryName"

Write-Host "Building agentic for windows/$Arch..."
& docker buildx build --build-arg "REPO_ROOT=$ScriptDir" --target export --output "$ScriptDir\dist\" $ScriptDir
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if (-not (Test-Path $BinarySrc)) {
  Write-Error "Expected binary not found at $BinarySrc"
  exit 1
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item $BinarySrc $InstallPath -Force
Write-Host "Installed agentic to $InstallPath"

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
  [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
  Write-Host "Added $InstallDir to user PATH"
  Write-Host "Note: restart your terminal for the PATH change to take effect"
}
