param([ValidatePattern('^[A-Za-z0-9][A-Za-z0-9-]{0,48}$')][string]$PreviewTag)
$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$binaryDir = Join-Path $taskRoot 'build/bin'
foreach ($required in @('MarkdownReader.exe', 'runtime/msedgewebview2.exe', 'tools/pandoc.exe')) {
    if (-not (Test-Path -LiteralPath (Join-Path $binaryDir $required))) { throw "缺少便携依赖：$required" }
}
$stageName = if ($PreviewTag) { 'MarkdownReader-preview-' + $PreviewTag } else { 'MarkdownReader' }
$stageDir = Join-Path (Join-Path $taskRoot 'dist') $stageName
New-Item -ItemType Directory -Force $stageDir | Out-Null
Copy-Item -LiteralPath (Join-Path $binaryDir 'MarkdownReader.exe') -Destination $stageDir -Force
foreach ($folderName in @('runtime','tools')) {
    & robocopy.exe (Join-Path $binaryDir $folderName) (Join-Path $stageDir $folderName) /E /NFL /NDL /NJH /NJS /NP /R:2 /W:1 | Out-Null
    if ($LASTEXITCODE -gt 7) { throw "复制 $folderName 失败" }
}
foreach ($folderName in @('docs','examples','licenses')) {
    & robocopy.exe (Join-Path $taskRoot $folderName) (Join-Path $stageDir $folderName) /E /NFL /NDL /NJH /NJS /NP /R:2 /W:1 /XF webview-download-page.html 'export-test-*' UI操作验证.pdf | Out-Null
    if ($LASTEXITCODE -gt 7) { throw "复制 $folderName 失败" }
}
Copy-Item -LiteralPath (Join-Path $taskRoot 'README.md') -Destination $stageDir -Force
Copy-Item -LiteralPath (Join-Path $taskRoot 'build/windows/icon.ico') -Destination (Join-Path $stageDir 'MarkdownReader.ico') -Force
Copy-Item -LiteralPath (Join-Path $taskRoot 'MarkdownReader.png') -Destination $stageDir -Force
if ($PreviewTag) { Copy-Item -LiteralPath (Join-Path $taskRoot 'docs/预览版说明.md') -Destination (Join-Path $stageDir 'PREVIEW.md') -Force }
$zipName = if ($PreviewTag) { 'MarkdownReader-0.1.0-preview-' + $PreviewTag + '-windows-x64.zip' } else { 'MarkdownReader-0.1.0-windows-x64-portable.zip' }
$zipPath = Join-Path (Join-Path $taskRoot 'dist') $zipName
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem
$taskZipTemp = $zipPath + '.building-' + [guid]::NewGuid().ToString('N')
$taskArchive = $null
function Add-PortableFile([System.IO.FileInfo]$taskFile) {
    if (($taskFile.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { return }
    if ($taskFile.Name -like 'export-test-*' -or $taskFile.Name -in @('webview-download-page.html','UI操作验证.pdf')) { return }
    $taskRelative = $taskFile.FullName.Substring($stageDir.Length).TrimStart([IO.Path]::DirectorySeparatorChar).Replace('\','/')
    [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($taskArchive, $taskFile.FullName, ('MarkdownReader/' + $taskRelative), [IO.Compression.CompressionLevel]::Optimal) | Out-Null
}
function Add-PortableDirectory([string]$taskDirectory) {
    foreach ($taskItem in Get-ChildItem -LiteralPath $taskDirectory -Force) {
        if (($taskItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { continue }
        if ($taskItem.PSIsContainer) { Add-PortableDirectory $taskItem.FullName } else { Add-PortableFile $taskItem }
    }
}
try {
    $taskArchive = [IO.Compression.ZipFile]::Open($taskZipTemp, [IO.Compression.ZipArchiveMode]::Create)
    foreach ($taskFileName in @('MarkdownReader.exe','MarkdownReader.ico','MarkdownReader.png','README.md')) {
        Add-PortableFile (Get-Item -LiteralPath (Join-Path $stageDir $taskFileName))
    }
    if ($PreviewTag) { Add-PortableFile (Get-Item -LiteralPath (Join-Path $stageDir 'PREVIEW.md')) }
    foreach ($taskFolderName in @('runtime','tools','docs','examples','licenses')) {
        Add-PortableDirectory (Join-Path $stageDir $taskFolderName)
    }
    $taskArchive.Dispose(); $taskArchive = $null
    Move-Item -LiteralPath $taskZipTemp -Destination $zipPath -Force
} finally {
    if ($null -ne $taskArchive) { $taskArchive.Dispose() }
    if (Test-Path -LiteralPath $taskZipTemp) { Remove-Item -LiteralPath $taskZipTemp -Force }
}
(Get-FileHash -LiteralPath $zipPath -Algorithm SHA256).Hash | Set-Content -LiteralPath ($zipPath + '.sha256') -Encoding ascii
Write-Output $zipPath
