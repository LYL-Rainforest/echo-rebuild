$ErrorActionPreference = "Stop"
$root = "E:\appdata\code\echo-rebuild"
Set-Location $root
$pkg = "internal/store"
$ok = $true

foreach ($f in @("models.go","db.go")) {
    $path = "$root\$pkg\$f"
    if (Test-Path $path) { Write-Host "  ✓ $pkg/$f" -ForegroundColor Green }
    else { Write-Host "  ✗ $pkg/$f (missing)" -ForegroundColor Red; $ok = $false }
}

$err = & go build "./$pkg/..." 2>&1
if ($LASTEXITCODE -eq 0) { Write-Host "  ✓ $pkg build" -ForegroundColor Green } else { Write-Host "  ✗ $pkg build`n$err" -ForegroundColor Red; $ok = $false }

$err = & go vet "./$pkg/..." 2>&1
if ($LASTEXITCODE -eq 0) { Write-Host "  ✓ $pkg vet" -ForegroundColor Green } else { Write-Host "  ✗ $pkg vet`n$err" -ForegroundColor Red; $ok = $false }

$err = & go test "./$pkg/..." 2>&1
if ($LASTEXITCODE -eq 0) { Write-Host "  ✓ $pkg test" -ForegroundColor Green } else { Write-Host "  ✗ $pkg test`n$err" -ForegroundColor Red; $ok = $false }

if ($ok) { Write-Host "`n  STORE — 正常" -ForegroundColor Green } else { Write-Host "`n  STORE — 不正常" -ForegroundColor Red }
exit $(if ($ok) {0} else {1})
