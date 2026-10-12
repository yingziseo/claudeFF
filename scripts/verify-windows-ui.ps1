param([Parameter(Mandatory=$true)][string]$Executable,[Parameter(Mandatory=$true)][string]$Output)
$ErrorActionPreference='Stop'
$exe=(Resolve-Path -LiteralPath $Executable).Path
New-Item -ItemType Directory -Force $Output | Out-Null
$env:PRIVACY_UI_TEST_OUTPUT=(Resolve-Path -LiteralPath $Output).Path
$env:DOTNET_HOST_TRACE='1'
$env:DOTNET_HOST_TRACEFILE=Join-Path $env:PRIVACY_UI_TEST_OUTPUT 'host-trace.txt'
$process=$null
try {
    $process=Start-Process -FilePath $exe -ArgumentList '--ui-smoke-test' -WorkingDirectory (Split-Path $exe) -PassThru -RedirectStandardOutput (Join-Path $Output 'stdout.txt') -RedirectStandardError (Join-Path $Output 'stderr.txt')
    $deadline=[DateTime]::UtcNow.AddSeconds(40)
    $observedWindow=$false
    while (-not $process.HasExited -and [DateTime]::UtcNow -lt $deadline) {
        $process.Refresh()
        if ($process.MainWindowHandle -ne [IntPtr]::Zero) {$observedWindow=$true}
        Start-Sleep -Milliseconds 100
    }
    if (-not $process.HasExited) {$process.Kill();throw 'Native startup timed out.'}
    if ($process.ExitCode -ne 0) {throw "Native startup failed: exit code $($process.ExitCode)"}
    $result=Get-Content (Join-Path $Output 'ui-result.json') -Raw | ConvertFrom-Json
    if (-not $result.Success -or $result.Checks.Count -ne 8) {throw 'Expected 8 native page/theme checks.'}
    foreach ($check in $result.Checks) {
        if (-not $check.Visible -or $check.WindowHandle -eq 0) {throw 'An expected native window was not visible.'}
        if (-not (Test-Path (Join-Path $Output $check.Screenshot))) {throw 'Native screenshot missing.'}
    }
    Write-Host "PASS: executable started; eight native page/theme checks; screenshots captured; external window observed=$observedWindow"
    if (-not $observedWindow) {throw 'The verifier did not observe a native top-level window.'}
} finally {
    Get-ChildItem -LiteralPath $Output -Filter '*.log' | ForEach-Object {Get-Content $_.FullName}
    if ($process -and -not $process.HasExited) {$process.Kill()}
    Remove-Item Env:PRIVACY_UI_TEST_OUTPUT -ErrorAction SilentlyContinue
    Remove-Item Env:DOTNET_HOST_TRACE -ErrorAction SilentlyContinue
    Remove-Item Env:DOTNET_HOST_TRACEFILE -ErrorAction SilentlyContinue
}
