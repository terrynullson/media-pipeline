@echo off
setlocal
powershell.exe -ExecutionPolicy Bypass -File "%~dp0install_transcription_backend.ps1" %*
