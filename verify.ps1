$ErrorActionPreference = "Stop"
$root = "E:\appdata\code\echo-rebuild"

function Write-Green { Write-Host "  $($args[0])" -ForegroundColor Green }
function Write-Red   { Write-Host "  ✗ $($args[0])" -ForegroundColor Red }
function Write-Blank  { Write-Host "" }
function Write-Section { Write-Host "`n── $($args[0]) ────────────────────────────────" -ForegroundColor Yellow }

Set-Location -LiteralPath $root

Write-Host "`n==============================================" -ForegroundColor Cyan
Write-Host "  EchoRebuild — 模块检验" -ForegroundColor Cyan
Write-Host "==============================================" -ForegroundColor Cyan

# ─── 1. 文件清单 ───
Write-Section "1. File Manifest"

$files = @(
    "internal/store/models.go",
    "internal/store/db.go",
    "internal/scanner/scanner.go",
    "internal/scanner/scanner_windows.go",
    "internal/scanner/scanner_unix.go",
    "internal/engine/pool.go",
    "internal/engine/installer.go",
    "internal/app/workflow.go",
    "cmd/echo/main.go",
    "cmd/echo/model.go",
    "cmd/echo/styles.go",
    "cmd/echo/pages/menu.go",
    "cmd/echo/pages/config_backup.go",
    "cmd/echo/pages/config_restore.go",
    "cmd/echo/pages/progress.go",
    "cmd/echo/pages/result.go",
    "cmd/echo/shared/input.go",
    "cmd/echo/shared/disk_list.go",
    "cmd/echo/shared/progress_bar.go",
    "cmd/echo/msg/types.go",
    "echo-rebuilt_spec.md",
    "build.sh",
    "go.mod",
    "verify.ps1"
)

$ok = $true
foreach ($f in $files) {
    if (Test-Path -LiteralPath "$root\$f") {
        Write-Green "✓ $f"
    } else {
        Write-Red "✗ $f  (missing)"
        $ok = $false
    }
}
if ($ok) { Write-Green "`n  所有文件存在" }

# ─── 2. Go 编译 ───
Write-Section "2. Compile"

$res = & go build ./internal/store/... 2>&1
if ($LASTEXITCODE -eq 0) { Write-Green "✓ internal/store" } else { Write-Red "internal/store`n$res"; $ok=$false }

$res = & go build ./internal/engine/... 2>&1
if ($LASTEXITCODE -eq 0) { Write-Green "✓ internal/engine" } else { Write-Red "internal/engine`n$res"; $ok=$false }

$res = & go build ./internal/scanner/... 2>&1
if ($LASTEXITCODE -eq 0) { Write-Green "✓ internal/scanner" } else { Write-Red "internal/scanner`n$res"; $ok=$false }

$res = & go build ./internal/app/... 2>&1
if ($LASTEXITCODE -eq 0) { Write-Green "✓ internal/app" } else { Write-Red "internal/app`n$res"; $ok=$false }

$res = & go build ./cmd/echo/... 2>&1
if ($LASTEXITCODE -eq 0) { Write-Green "✓ cmd/echo" } else { Write-Red "cmd/echo`n$res"; $ok=$false }

$res = & go vet ./... 2>&1
if ($LASTEXITCODE -eq 0) { Write-Green "✓ go vet ./..." } else { Write-Red "go vet ./...`n$res"; $ok=$false }

# ─── 3. 交叉编译 ───
Write-Section "3. Cross-compile"

$env:CGO_ENABLED = "0"
$env:GOARCH = "amd64"

$env:GOOS = "windows"
$res = & go build -o build\verify-windows-amd64.exe ./cmd/echo/ 2>&1
if ($LASTEXITCODE -eq 0) { Write-Green "✓ windows/amd64" } else { Write-Red "windows/amd64`n$res"; $ok=$false }

$env:GOOS = "linux"
$res = & go build -o build\verify-linux-amd64 ./cmd/echo/ 2>&1
if ($LASTEXITCODE -eq 0) { Write-Green "✓ linux/amd64" } else { Write-Red "linux/amd64`n$res"; $ok=$false }

$env:GOOS = "darwin"
$res = & go build -o build\verify-darwin-amd64 ./cmd/echo/ 2>&1
if ($LASTEXITCODE -eq 0) { Write-Green "✓ darwin/amd64" } else { Write-Red "darwin/amd64`n$res"; $ok=$false }

# cleanup
Remove-Item -Force "build\verify-windows-amd64.exe", "build\verify-linux-amd64", "build\verify-darwin-amd64" -ErrorAction SilentlyContinue
$env:CGO_ENABLED = ""; $env:GOOS = ""; $env:GOARCH = ""

# ─── 汇总 ───
Write-Host "`n==============================================" -ForegroundColor Cyan
if ($ok) {
    Write-Green "  所有模块检验通过"
} else {
    Write-Red "  存在未通过的检验项"
}
Write-Host "==============================================" -ForegroundColor Cyan
