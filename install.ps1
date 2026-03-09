#Requires -Version 5.1

$ErrorActionPreference = "Stop"

$Repo = $env:CALLMEBACK_REPO
if ([string]::IsNullOrWhiteSpace($Repo)) { $Repo = "tungnguyensipher/callmeback" }

$Version = $env:CALLMEBACK_VERSION
$InstallDir = $env:CALLMEBACK_INSTALL_DIR
if ([string]::IsNullOrWhiteSpace($InstallDir)) {
  $InstallDir = Join-Path $env:LOCALAPPDATA "callmeback\\bin"
}

$Arch = $env:PROCESSOR_ARCHITECTURE
if ($Arch -eq "AMD64") { $GoArch = "amd64" }
else { throw "Unsupported architecture: $Arch (only windows/amd64 is published)" }

$GoOs = "windows"

function Get-LatestVersion {
  $latestUrl = "https://github.com/$Repo/releases/latest"
  try {
    $resp = Invoke-WebRequest -Uri $latestUrl -MaximumRedirection 10 -Headers @{ "User-Agent" = "callmeback-installer" }
    $finalUrl = $resp.BaseResponse.ResponseUri.AbsoluteUri
    if ($finalUrl -match "/tag/v([^/]+)$") { return $Matches[1] }
  } catch {
  }

  $apiUrl = "https://api.github.com/repos/$Repo/releases/latest"
  $apiResp = Invoke-RestMethod -Uri $apiUrl -Headers @{ "User-Agent" = "callmeback-installer"; "Accept" = "application/vnd.github+json" }
  if (-not $apiResp.tag_name) { throw "No tag_name in GitHub response" }
  return ($apiResp.tag_name -replace "^v", "")
}

if ([string]::IsNullOrWhiteSpace($Version)) {
  $Version = Get-LatestVersion
}

if ([string]::IsNullOrWhiteSpace($Version)) {
  throw "Failed to detect latest version. Set CALLMEBACK_VERSION=1.2.3 and retry."
}

$Tag = "v$Version"
$Asset = "callmeback_${Version}_${GoOs}_${GoArch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Tag/$Asset"

Write-Host "Installing callmeback $Tag from $Url"

$TempDir = Join-Path $env:TEMP ("callmeback-install-" + [guid]::NewGuid().ToString("n"))
New-Item -ItemType Directory -Path $TempDir | Out-Null

try {
  $ZipPath = Join-Path $TempDir $Asset
  Invoke-WebRequest -Uri $Url -OutFile $ZipPath
  Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force

  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  Copy-Item -Path (Join-Path $TempDir "callmeback.exe") -Destination (Join-Path $InstallDir "callmeback.exe") -Force

  $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
  if ($null -eq $currentPath) { $currentPath = "" }
  $pathParts = $currentPath -split ";" | Where-Object { $_ -and $_.Trim() -ne "" }
  if ($pathParts -notcontains $InstallDir) {
    $newPath = ($pathParts + $InstallDir) -join ";"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    $env:Path = $newPath + ";" + $env:Path
    Write-Host "Added to PATH (User): $InstallDir"
    Write-Host "Restart your terminal for PATH changes to take effect everywhere."
  }

  Write-Host "Installed: $InstallDir\\callmeback.exe"
  Write-Host "Run: callmeback --help"
}
finally {
  Remove-Item -Recurse -Force $TempDir -ErrorAction SilentlyContinue
}
