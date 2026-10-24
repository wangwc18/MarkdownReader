$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$vendorDir = Join-Path $taskRoot 'frontend/public/vendor/vditor'
New-Item -ItemType Directory -Force $vendorDir | Out-Null
$assetSource = Join-Path $taskRoot 'frontend/node_modules/vditor/dist'
$assetTarget = Join-Path $vendorDir 'dist'
& robocopy.exe $assetSource $assetTarget /E /NFL /NDL /NJH /NJS /NP /R:2 /W:1 | Out-Null
if ($LASTEXITCODE -gt 7) { throw '编辑器资源复制失败' }
& node (Join-Path $PSScriptRoot 'patch-lute.mjs')
if ($LASTEXITCODE -ne 0) { throw 'Lute 兼容补丁应用失败' }
$licenseDir = Join-Path $taskRoot 'licenses'
New-Item -ItemType Directory -Force $licenseDir | Out-Null
Copy-Item -LiteralPath (Join-Path $taskRoot 'frontend/node_modules/vditor/LICENSE') -Destination (Join-Path $licenseDir 'Vditor-LICENSE.txt') -Force

Copy-Item -LiteralPath (Join-Path $taskRoot 'frontend/node_modules/nspell/license') -Destination (Join-Path $licenseDir 'Nspell-LICENSE.txt') -Force
Copy-Item -LiteralPath (Join-Path $taskRoot 'frontend/node_modules/dictionary-en/license') -Destination (Join-Path $licenseDir 'EnglishDictionary-LICENSE.txt') -Force
Copy-Item -LiteralPath (Join-Path $taskRoot 'frontend/node_modules/is-buffer/LICENSE') -Destination (Join-Path $licenseDir 'IsBuffer-LICENSE.txt') -Force

Copy-Item -LiteralPath (Join-Path $taskRoot 'frontend/node_modules/dictionary-en-gb/license') -Destination (Join-Path $licenseDir 'BritishDictionary-LICENSE.txt') -Force
