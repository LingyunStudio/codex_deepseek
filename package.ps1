# CodeSeek Full Build + Installer
# Run .\build.ps1 first to create codeseek-gui.exe

$ErrorActionPreference = "Stop"

# Check for Inno Setup
$iscc = "D:\InnoSetup6\ISCC.exe"
if (-not (Test-Path $iscc)) {
    Write-Error "Inno Setup not found. Install from: https://jrsoftware.org/isdl.php"
    exit 1
}

# Require the exe to exist
$exe = Join-Path $PSScriptRoot "codeseek-gui.exe"
if (-not (Test-Path $exe)) {
    Write-Error "codeseek-gui.exe not found. Run .\build.ps1 first."
    exit 1
}

# Build installer
New-Item -ItemType Directory -Force -Path (Join-Path $PSScriptRoot "dist") | Out-Null
& $iscc (Join-Path $PSScriptRoot "setup.iss")
if ($LASTEXITCODE -eq 0) {
    $setup = Get-ChildItem (Join-Path $PSScriptRoot "dist") -Filter "CodeSeek-Setup-*.exe" | Sort-Object LastWriteTime -Descending | Select-Object -First 1
    Write-Host "Installer OK: $($setup.FullName)" -ForegroundColor Green
}
