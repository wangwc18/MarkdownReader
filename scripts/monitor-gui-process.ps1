param(
    [Parameter(Mandatory=$true)][string]$Executable,
    [Parameter(Mandatory=$true)][string]$ReportPrefix,
    [int]$ObservationSeconds = 300,
    [switch]$SystemPathOnly
)
$ErrorActionPreference = 'Stop'
$taskOriginalPath = $env:PATH
try {
    if ($SystemPathOnly) { $env:PATH = $env:SystemRoot + '\System32;' + $env:SystemRoot }
    $observedApp = Start-Process -FilePath $Executable -PassThru -WindowStyle Normal
} finally { $env:PATH = $taskOriginalPath }
# Open and retain the handle before an exit can race a later Get-Process call.
$observedHandle = $observedApp.Handle
@{pid=$observedApp.Id; startedUtc=[DateTime]::UtcNow.ToString('o'); handleHeld=($observedHandle -ne [IntPtr]::Zero); systemPathOnly=[bool]$SystemPathOnly} |
    ConvertTo-Json | Set-Content -LiteralPath ($ReportPrefix + '-start.json') -Encoding utf8
Write-Output ('pid=' + $observedApp.Id)
$ended = $observedApp.WaitForExit($ObservationSeconds * 1000)
$observation = @{pid=$observedApp.Id; checkedUtc=[DateTime]::UtcNow.ToString('o'); ended=$ended}
if ($ended) { $observation.exitCode = $observedApp.ExitCode }
else { $observedApp.Refresh(); $observation.responding = $observedApp.Responding }
$observation | ConvertTo-Json | Set-Content -LiteralPath ($ReportPrefix + '-result.json') -Encoding utf8
$observation | ConvertTo-Json
$observedApp.Dispose()
