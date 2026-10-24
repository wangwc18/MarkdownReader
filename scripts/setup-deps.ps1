param([string]$DependencyCache, [switch]$Offline)
$ErrorActionPreference = 'Stop'
if ($DependencyCache) { $DependencyCache = (Resolve-Path -LiteralPath $DependencyCache).Path }
$taskRoot = Split-Path -Parent $PSScriptRoot
Push-Location $taskRoot
try {
    New-Item -ItemType Directory -Force downloads,build/bin/tools,downloads/webview-extracted | Out-Null
    $taskDependencies = @(
        @{File='downloads/webview2-155.0.4283.45-x64.cab'; Url='https://msedge.sf.dl.delivery.mp.microsoft.com/filestreamingservice/files/01b0fbd9-6ea3-4e12-818a-a71f73f3855b/Microsoft.WebView2.FixedVersionRuntime.155.0.4283.45.x64.cab'; Hash='EC533BF1B7354D288703CF821A2D640D0130108224F93E330BB9C09FAF3F8010'},
        @{File='downloads/pandoc-3.12.1.zip'; Url='https://github.com/jgm/pandoc/releases/download/3.12.1/pandoc-3.12.1-windows-x86_64.zip'; Hash='6EF431CB20B2C24A0371FC19200DC49D452BE54B1B5C9239F3462841EF53E587'}
    )
    foreach($dependency in $taskDependencies) {
        if(-not (Test-Path -LiteralPath $dependency.File)) {
            $cachedFile = if ($DependencyCache) { Join-Path $DependencyCache (Split-Path $dependency.File -Leaf) }
            if ($cachedFile -and (Test-Path -LiteralPath $cachedFile)) {
                Copy-Item -LiteralPath $cachedFile -Destination $dependency.File
            } elseif ($Offline) {
                throw ('Dependency cache missing: ' + $dependency.File + '; see RECOVERY.md')
            } else {
                Invoke-WebRequest -Uri $dependency.Url -OutFile $dependency.File
            }
        }
        if((Get-FileHash -LiteralPath $dependency.File -Algorithm SHA256).Hash -ne $dependency.Hash) { throw ('依赖校验失败：'+$dependency.File) }
    }
    & expand.exe downloads/webview2-155.0.4283.45-x64.cab '-F:*' downloads/webview-extracted | Out-Null
    if($LASTEXITCODE -ne 0){throw 'WebView2 解压失败'}
    & robocopy.exe downloads/webview-extracted/Microsoft.WebView2.FixedVersionRuntime.155.0.4283.45.x64 build/bin/runtime /E /NFL /NDL /NJH /NJS /NP /R:2 /W:1 | Out-Null
    if($LASTEXITCODE -gt 7){throw 'WebView2 复制失败'}
    Expand-Archive -LiteralPath downloads/pandoc-3.12.1.zip -DestinationPath downloads/pandoc -Force
    Copy-Item -LiteralPath downloads/pandoc/pandoc-3.12.1/pandoc.exe -Destination build/bin/tools/pandoc.exe -Force
} finally { Pop-Location }
