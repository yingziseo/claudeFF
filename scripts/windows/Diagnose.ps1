$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
$run = Join-Path $env:LOCALAPPDATA ('ClaudeFF\Logs\diagnose-' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
New-Item -ItemType Directory -Force -Path $run | Out-Null
$report = Join-Path $run 'diagnosis.txt'
function Report([string]$message) {
    $message | Tee-Object -FilePath $report -Append | Write-Host
}
try {
    Report 'claudeFF startup diagnosis - no privacy rules will be changed.'
    Report ('Package: ' + $root)
    $os = Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion'
    Report ('Windows: ' + $os.ProductName + ' ' + $os.DisplayVersion + '; build=' + $os.CurrentBuildNumber + '.' + $os.UBR)
    Report ('64-bit OS: ' + [Environment]::Is64BitOperatingSystem)
    if (-not [Environment]::Is64BitOperatingSystem) { throw 'This package requires 64-bit Windows. It cannot run on 32-bit Windows.' }
    $required = @('ClaudePrivacy.exe','ClaudePrivacy.dll','ClaudePrivacy.deps.json','ClaudePrivacy.runtimeconfig.json','coreclr.dll','hostfxr.dll','hostpolicy.dll','PresentationFramework.dll','WindowsBase.dll','SHA256SUMS.txt')
    foreach ($file in $required) {
        if (-not (Test-Path -LiteralPath (Join-Path $root $file))) { throw ('Missing file: ' + $file + '. Extract the entire ZIP to a new folder first.') }
    }
    Report 'Checking package hashes...'
    foreach ($line in Get-Content -LiteralPath (Join-Path $root 'SHA256SUMS.txt')) {
        if ($line -match '^([0-9a-fA-F]{64})  (.+)$') {
            $expected=$Matches[1]; $relative=$Matches[2]
            $file = Join-Path $root $relative
            if (-not (Test-Path -LiteralPath $file)) { throw ('Missing package file: ' + $relative) }
            if ((Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash -ne $expected) { throw ('Package file changed: ' + $relative + '. Re-extract the ZIP to a new folder.') }
        }
    }
    Report 'Package file hashes passed.'
    $env:DOTNET_HOST_TRACE='1';$env:DOTNET_HOST_TRACEFILE=Join-Path $run 'host-trace.txt'
    $env:COREHOST_TRACE='1';$env:COREHOST_TRACEFILE=$env:DOTNET_HOST_TRACEFILE
    $p=Start-Process -FilePath (Join-Path $root 'ClaudePrivacy.exe') -WorkingDirectory $root -PassThru -RedirectStandardOutput (Join-Path $run 'stdout.txt') -RedirectStandardError (Join-Path $run 'stderr.txt')
    Report ('Started PID: ' + $p.Id)
    $visible=$false
    for ($i=0;$i -lt 40;$i++) {
        Start-Sleep -Milliseconds 250
        $p.Refresh()
        if ($p.HasExited) { break }
        if ($p.MainWindowHandle -ne [IntPtr]::Zero) { $visible=$true;break }
    }
    if ($p.HasExited) { Report ('Process exited. Code: ' + $p.ExitCode + ' (0x' + $p.ExitCode.ToString('X8') + ')') }
    elseif ($visible) { Report 'A window was detected. Check whether it is the main window or an error dialog.' }
    else { Report 'Process is running but no top-level window was detected after 10 seconds. It has not been terminated.' }
} catch { Report ('ERROR: ' + $_.Exception.Message) }
$startup = Join-Path $env:LOCALAPPDATA 'ClaudeFF\Logs\startup.log'
if (Test-Path -LiteralPath $startup) { Copy-Item -LiteralPath $startup -Destination (Join-Path $run 'startup.log') }
Report ('Diagnostic files: ' + $run)
Report 'Share diagnosis.txt and startup.log to investigate. Logs may contain local paths; review before sharing.'
Start-Process explorer.exe -ArgumentList ('"' + $run + '"')
