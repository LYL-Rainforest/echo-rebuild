$ErrorActionPreference = "Stop"
$root = "E:\appdata\code\echo-rebuild"
Set-Location $root
$pkg = "internal/scanner"
$ok = $true

foreach ($f in @("scanner.go","scanner_windows.go","scanner_unix.go")) {
    $path = "$root\$pkg\$f"
    if (Test-Path $path) { Write-Host "  ✓ $pkg/$f" -ForegroundColor Green }
    else { Write-Host "  ✗ $pkg/$f (missing)" -ForegroundColor Red; $ok = $false }
}

$err = & go build "./$pkg/..." 2>&1
if ($LASTEXITCODE -eq 0) { Write-Host "  ✓ $pkg build" -ForegroundColor Green } else { Write-Host "  ✗ $pkg build`n$err" -ForegroundColor Red; $ok = $false }

$err = & go vet "./$pkg/..." 2>&1
if ($LASTEXITCODE -eq 0) { Write-Host "  ✓ $pkg vet" -ForegroundColor Green } else { Write-Host "  ✗ $pkg vet`n$err" -ForegroundColor Red; $ok = $false }

if ($ok) { Write-Host "`n  SCANNER — 正常" -ForegroundColor Green } else { Write-Host "`n  SCANNER — 不正常" -ForegroundColor Red }
exit $(if ($ok) {0} else {1})
