$ErrorActionPreference = 'Stop'
if ($env:GITHUB_ACTIONS -ne 'true') { throw 'AGENTS.md requires building and packaging in GitHub Actions.' }
Set-Location (Join-Path $PSScriptRoot '..')
dotnet run --project tests/ClaudeFF.Tests -c Release
if ($LASTEXITCODE -ne 0) { throw 'Core tests failed.' }
$out = Join-Path (Get-Location) 'artifacts/claudeFF-win-x64'
New-Item -ItemType Directory -Force $out | Out-Null
foreach ($project in @('Probe', 'Relay', 'App')) {
    dotnet publish "src/ClaudeFF.$project" -c Release -r win-x64 --self-contained true -p:DebugType=None -p:DebugSymbols=false -o $out --nologo
    if ($LASTEXITCODE -ne 0) { throw "Publish failed: $project" }
}
Copy-Item docs/Windows使用说明.md (Join-Path $out '使用说明.md')
Copy-Item docs/第三方与接口来源.md $out
Copy-Item docs/实现与验证边界.md $out
Copy-Item docs/界面预览.html $out
Copy-Item docs/assets/LUCIDE-LICENSE.txt $out
Copy-Item scripts/windows/* $out
# Start the published EXE and require actual native windows and page/theme evidence.
& ./scripts/verify-windows-ui.ps1 -Executable (Join-Path $out 'ClaudePrivacy.exe') -Output artifacts/native-ui
@{Commit=$env:GITHUB_SHA;RunId=$env:GITHUB_RUN_ID;Version='0.1.2';MainExecutable='ClaudePrivacy.exe';Runtime=[System.Runtime.InteropServices.RuntimeInformation]::OSDescription} | ConvertTo-Json | Set-Content (Join-Path $out 'BUILD-INFO.json') -Encoding utf8
$manifest = Get-ChildItem $out -Recurse -File | Where-Object Name -ne 'SHA256SUMS.txt' | ForEach-Object {
    $hash = (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  $([IO.Path]::GetRelativePath($out, $_.FullName))"
}
$manifest | Set-Content (Join-Path $out 'SHA256SUMS.txt') -Encoding utf8
Compress-Archive -Path $out -DestinationPath artifacts/claudeFF-win-x64-v0.1.2.zip -Force
Get-FileHash artifacts/claudeFF-win-x64-v0.1.2.zip -Algorithm SHA256
