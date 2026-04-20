@echo off
chcp 65001 >nul 2>&1
setlocal

cd /d "%~dp0"
powershell.exe -ExecutionPolicy Bypass -File "%~dp0scripts\dev_down.ps1"
exit /b %ERRORLEVEL%
