param([switch]$Package)
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
Push-Location $taskRoot
try {
    & npm ci --prefix frontend
    if ($LASTEXITCODE -ne 0) { throw 'npm ci 失败' }
    & (Join-Path $PSScriptRoot 'prepare-assets.ps1')
    & node --test (Join-Path $PSScriptRoot 'lute-modes.test.mjs')
    if ($LASTEXITCODE -ne 0) { throw '解析器模式测试失败' }
    & npm test --prefix frontend
    if ($LASTEXITCODE -ne 0) { throw '前端编辑测试失败' }
    & npm run build --prefix frontend
    if ($LASTEXITCODE -ne 0) { throw '前端构建失败' }
    & go run ./scripts/icon
    if ($LASTEXITCODE -ne 0) { throw '图标构建失败' }
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw '测试失败' }
    & go build -tags production -trimpath -ldflags '-H windowsgui -s -w' -o build/bin/MarkdownReader.exe .
    if ($LASTEXITCODE -ne 0) { throw 'Go 构建失败' }
    if ($Package) { & (Join-Path $PSScriptRoot 'package.ps1') }
} finally { Pop-Location }
