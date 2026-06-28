# CodeSeek GUI Build Script
$ErrorActionPreference = "Stop"
$guiDir = Join-Path $PSScriptRoot "cmd\codeseek-gui"

# Step 1: Generate icons
Push-Location (Join-Path $PSScriptRoot "assets")
go run gen.go
Pop-Location

# Step 2: Copy tray icon into embedded assets
Copy-Item (Join-Path $PSScriptRoot "assets\icon-c-32.png") (Join-Path $guiDir "frontend\src\icon-tray.png") -Force
Copy-Item (Join-Path $PSScriptRoot "assets\icon-app.png") (Join-Path $guiDir "frontend\src\icon-app.png") -Force

# Step 3: Generate multi-res ICO and Windows resource (goversioninfo)
go run (Join-Path $PSScriptRoot "scripts\makeico.go")
Push-Location $guiDir
# Clean old .syso files, build icon resource with windres
Remove-Item *.syso -Force -ErrorAction SilentlyContinue
"1 ICON `"codeseek.ico`"" | Out-File -Encoding ASCII icon.rc
# Find windres (MinGW/MSYS2/anaconda)
$windres = Get-Command windres -ErrorAction SilentlyContinue
if (-not $windres) {
# 需要修改为自己的路径
    $paths = @("D:\anaconda\Library\mingw-w64\bin\windres.exe", "C:\msys64\mingw64\bin\windres.exe")
    foreach ($p in $paths) { if (Test-Path $p) { $windres = $p; break } }
}
if ($windres) {
    & $windres -i icon.rc -o codeseek.syso -O coff
    Remove-Item icon.rc
} else {
    throw "windres not found. Install MinGW or MSYS2."
}
Pop-Location

# Step 4: Generate Wails bindings
Push-Location $guiDir
wails3 generate bindings
if ($LASTEXITCODE -ne 0) { throw "Bindings generation failed" }

# Step 5: Fix @wailsio/runtime import for production
Get-ChildItem -Path "frontend\bindings" -Recurse -Filter "*.js" | ForEach-Object {
    $c = Get-Content $_.FullName -Raw
    if ($c -match '@wailsio/runtime') {
        Set-Content $_.FullName -Value ($c -replace '@wailsio/runtime', '/wails/runtime.js') -NoNewline
        Write-Host "Fixed: $($_.FullName)"
    }
}
Pop-Location

# Step 6: Build (must run from guiDir for .syso to be picked up)
Push-Location $guiDir
$out = Join-Path $PSScriptRoot "codeseek-gui.exe"
go build -ldflags="-H windowsgui -s -w" -o $out .
if ($LASTEXITCODE -eq 0) {
    Write-Host "Build OK: $out" -ForegroundColor Green
} else {
    throw "Build failed"
}
Pop-Location
