@echo off
setlocal
cd /d "%~dp0"
if not exist "ClaudePrivacy.dll" goto incomplete
if not exist "coreclr.dll" goto incomplete
start "" /wait "%~dp0ClaudePrivacy.exe"
if errorlevel 1 (
  echo ClaudeFF exited with an error. Run Diagnose.cmd in this folder.
  pause
)
exit /b
:incomplete
echo Extract the ENTIRE ZIP to a normal folder before starting ClaudeFF.
pause
exit /b 1
