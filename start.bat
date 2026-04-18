@echo off
chcp 65001 >nul 2>&1
title Media Pipeline

echo ============================================
echo   POGNALI NAHUI
echo ============================================
echo.

:: Check dependencies
where go >nul 2>&1 || (echo [ERROR] Go not found in PATH && pause && exit /b 1)
where ffmpeg >nul 2>&1 || (echo [WARNING] FFmpeg not found -- audio extraction will fail)

:: Find Python (Windows Store stub returns exit code 49, skip it)
set "PYTHON_BIN="
for %%P in (
    "C:\Users\ablee\AppData\Local\Programs\Python\Python313\python.exe"
    "C:\Users\ablee\AppData\Local\Programs\Python\Python312\python.exe"
    "C:\Users\ablee\AppData\Local\Programs\Python\Python311\python.exe"
    "C:\Python313\python.exe"
    "C:\Python312\python.exe"
) do (
    if exist %%P if not defined PYTHON_BIN set "PYTHON_BIN=%%~P"
)
if not defined PYTHON_BIN (
    echo [WARNING] Python not found -- transcription will fail
    echo          Install Python and faster-whisper, or set PYTHON_BINARY env var
) else (
    echo [OK] Python: %PYTHON_BIN%
)

:: Navigate to project root
cd /d "%~dp0"

:: Set Python binary for the Go worker
if defined PYTHON_BIN set "PYTHON_BINARY=%PYTHON_BIN%"

:: Build frontend if dist is missing
if not exist "frontend_v1\dist\index.html" (
    echo [BUILD] Building frontend_v1...
    cd frontend_v1
    if not exist node_modules (
        echo [BUILD] Installing npm dependencies...
        call npm install
    )
    call npx vite build
    cd ..
    echo [BUILD] Frontend built.
    echo.
)

:: Kill any existing process on port 8080
for /f "tokens=5" %%a in ('netstat -ano ^| findstr :8080 ^| findstr LISTENING 2^>nul') do (
    taskkill /F /PID %%a >nul 2>&1
)

:: Start Go backend (port 8080)
echo [START] Go backend on :8080
start /b "" cmd /c "go run ./cmd/web 2>&1"

:: Wait for backend to be ready
echo [WAIT] Waiting for backend...
:wait_backend
timeout /t 1 /nobreak >nul
curl -s http://127.0.0.1:8080/health >nul 2>&1
if errorlevel 1 goto wait_backend
echo [READY] Backend is up.

:: Start Go worker
echo [START] Go worker
start /b "" cmd /c "go run ./cmd/worker 2>&1"

echo.
echo ============================================
echo.
echo   App:    http://localhost:8080/app-v1/
echo   Health: http://localhost:8080/health
echo   Logs:   data\logs\
echo.
echo   Press Ctrl+C to stop all services
echo ============================================
echo.

:: Keep the window open
:loop
timeout /t 60 /nobreak >nul
goto loop
