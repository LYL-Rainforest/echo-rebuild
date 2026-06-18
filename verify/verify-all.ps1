$ErrorActionPreference = "Continue"
$root = "E:\appdata\code\echo-rebuild"
$scripts = @(
    "verify-store.ps1",
    "verify-scanner.ps1",
    "verify-engine.ps1",
    "verify-app.ps1",
    "verify-cmd.ps1"
)

$verifyDir = "$root\verify"

Write-Host "==============================" -ForegroundColor Cyan
Write-Host " EchoRebuild — 逐模块检验" -ForegroundColor Cyan
Write-Host "==============================" -ForegroundColor Cyan

$allOk = $true
foreach ($s in $scripts) {
    $name = $s -replace "^verify-","" -replace "\.ps1$",""
    $fullPath = "$verifyDir\$s"
    Write-Host "`n── $name ──" -ForegroundColor Yellow
    & $fullPath
    if ($LASTEXITCODE -ne 0) { $allOk = $false }
}

Write-Host "`n==============================" -ForegroundColor Cyan
if ($allOk) { Write-Host "  ALL — 全部正常" -ForegroundColor Green }
else { Write-Host "  ALL — 存在不正常" -ForegroundColor Red }
Write-Host "==============================" -ForegroundColor Cyan
