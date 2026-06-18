$ErrorActionPreference = "Stop"
$root = "E:\appdata\code\echo-rebuild"
Set-Location $root
$pkg = "internal/app"
$ok = $true

$path = "$root\$pkg\workflow.go"
if (Test-Path $path) { Write-Host "  ✓ $pkg/workflow.go" -ForegroundColor Green }
else { Write-Host "  ✗ $pkg/workflow.go (missing)" -ForegroundColor Red; $ok = $false }

$err = & go build "./$pkg/..." 2>&1
if ($LASTEXITCODE -eq 0) { Write-Host "  ✓ $pkg build" -ForegroundColor Green } else { Write-Host "  ✗ $pkg build`n$err" -ForegroundColor Red; $ok = $false }

$err = & go vet "./$pkg/..." 2>&1
if ($LASTEXITCODE -eq 0) { Write-Host "  ✓ $pkg vet" -ForegroundColor Green } else { Write-Host "  ✗ $pkg vet`n$err" -ForegroundColor Red; $ok = $false }

$err = & go test "./$pkg/..." 2>&1
if ($LASTEXITCODE -eq 0) { Write-Host "  ✓ $pkg test" -ForegroundColor Green } else { Write-Host "  ✗ $pkg test`n$err" -ForegroundColor Red; $ok = $false }

if ($ok) { Write-Host "`n  APP — 正常" -ForegroundColor Green } else { Write-Host "`n  APP — 不正常" -ForegroundColor Red }
exit $(if ($ok) {0} else {1})
