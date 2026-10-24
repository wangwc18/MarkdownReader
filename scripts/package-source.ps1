$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$sourceStage = Join-Path $taskRoot 'dist/source-staging/MarkdownReader-source'
New-Item -ItemType Directory -Force $sourceStage | Out-Null
foreach ($fileName in @('go.mod','go.sum','wails.json','README.md','RECOVERY.md','CHANGELOG.md','.gitignore','.gitattributes','MarkdownReader.png')) {
    Copy-Item -LiteralPath (Join-Path $taskRoot $fileName) -Destination $sourceStage -Force
}
Get-ChildItem -LiteralPath $taskRoot -Filter '*.go' -File | ForEach-Object { Copy-Item -LiteralPath $_.FullName -Destination $sourceStage -Force }
foreach ($folderName in @('scripts','third_party','licenses','docs','examples')) {
    & robocopy.exe (Join-Path $taskRoot $folderName) (Join-Path $sourceStage $folderName) /E /NFL /NDL /NJH /NJS /NP /R:2 /W:1 /XF webview-download-page.html 'export-test-*' UI操作验证.pdf | Out-Null
    if ($LASTEXITCODE -gt 7) { throw "复制 $folderName 失败" }
}
foreach ($folderName in @('frontend/src','build/windows')) {
    & robocopy.exe (Join-Path $taskRoot $folderName) (Join-Path $sourceStage $folderName) /E /NFL /NDL /NJH /NJS /NP /R:2 /W:1 | Out-Null
    if ($LASTEXITCODE -gt 7) { throw "复制 $folderName 失败" }
}
New-Item -ItemType Directory -Force (Join-Path $sourceStage 'frontend') | Out-Null
foreach ($fileName in @('package.json','package-lock.json','index.html','vite.config.js')) {
    Copy-Item -LiteralPath (Join-Path $taskRoot "frontend/$fileName") -Destination (Join-Path $sourceStage 'frontend') -Force
}
$zipPath = Join-Path $taskRoot 'dist/MarkdownReader-0.1.0-source.zip'
Compress-Archive -LiteralPath $sourceStage -DestinationPath $zipPath -Force
(Get-FileHash -LiteralPath $zipPath -Algorithm SHA256).Hash | Set-Content -LiteralPath ($zipPath + '.sha256') -Encoding ascii
Write-Output $zipPath
