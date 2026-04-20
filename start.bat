@echo off
chcp 65001 >nul 2>&1
setlocal

cd /d "%~dp0"

echo ============================================
echo   Media Pipeline
echo ============================================
echo.
echo Required before launch:
echo   1. Configure DATABASE_URL or DB_HOST/DB_NAME/DB_USER
echo   2. Ensure PostgreSQL is reachable
echo   3. Ensure ffmpeg and Python are installed
echo.
echo start.bat opens two PowerShell windows:
echo   - web
echo   - worker (after web health check)
echo Stop command:
echo   - stop.bat
echo.

powershell.exe -ExecutionPolicy Bypass -File "%~dp0scripts\dev_up.ps1"
set "exit_code=%ERRORLEVEL%"

if not "%exit_code%"=="0" (
    echo.
    echo [ERROR] Startup failed. Review the message above and data\logs\*.log.
)

exit /b %exit_code%
