#!/usr/bin/env pwsh
# Debug script for immutablecheck analyzer

Write-Host "=== Immutablecheck Analyzer Debug ===" -ForegroundColor Cyan

# 1. 加载 descriptor set 测试
Write-Host "`n[1] Testing Descriptor Set Loading..." -ForegroundColor Yellow
Push-Location g:\test\goci-const-check
$descriptorPath = "pb/descriptor/all.protos.pb"
if (Test-Path $descriptorPath) {
    Write-Host "✓ Descriptor set found" -ForegroundColor Green
    $size = (Get-Item $descriptorPath).Length
    Write-Host "  Size: $size bytes" -ForegroundColor Gray
} else {
    Write-Host "✗ Descriptor set not found at: $descriptorPath" -ForegroundColor Red
}

# 2. 运行单元测试
Write-Host "`n[2] Running Unit Tests..." -ForegroundColor Yellow
Push-Location g:\test\goci-const-check\cmd\immitablecheck
go test -v -run TestLoadDescriptorSet 2>&1
go test -v -run TestSnakeToCamelCase 2>&1
go test -v -run TestAnalyzeMainFile 2>&1

# 3. 重新编译分析器
Write-Host "`n[3] Rebuilding Analyzer..." -ForegroundColor Yellow
Push-Location g:\test\goci-const-check
go install -v ./cmd/immitablecheck 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Build successful" -ForegroundColor Green
} else {
    Write-Host "✗ Build failed" -ForegroundColor Red
}

# 4. 运行分析
Write-Host "`n[4] Running Analysis on main.go..." -ForegroundColor Yellow
immitablecheck ./main.go 2>&1

# 5. 运行完整分析
Write-Host "`n[5] Running Full Analysis..." -ForegroundColor Yellow
immitablecheck ./... 2>&1 | Select-String -Pattern "\.go:" | Where-Object { $_ -notmatch "pbtagger" }

Write-Host "`n=== Debug Complete ===" -ForegroundColor Cyan
