@echo off
setlocal
powershell.exe -ExecutionPolicy Bypass -File "%~dp0dev_up.ps1" %*
