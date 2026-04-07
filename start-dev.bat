@echo off
chcp 65001 >nul 2>&1
title Media Pipeline -- Dev Mode

echo ============================================
echo   Media Pipeline -- Dev Mode (hot reload)
echo ============================================
echo.

:: Check dependencies
where go >nul 2>&1 || (echo [ERROR] Go not found in PATH && pause && exit /b 1)
where node >nul 2>&1 || (echo [ERROR] Node.js not found in PATH && pause && exit /b 1)

:: Find Python
set "PYTHON_BIN="
for %%P in (
    "C:\Users\ablee\AppData\Local\Programs\Python\Python313\python.exe"
    "C:\Users\ablee\AppData\Local\Programs\Python\Python312\python.exe"
    "C:\Users\ablee\AppData\Local\Programs\Python\Python311\python.exe"
) do (
    if exist %%P if not defined PYTHON_BIN set "PYTHON_BIN=%%~P"
)
if defined PYTHON_BIN (
    echo [OK] Python: %PYTHON_BIN%
    set "PYTHON_BINARY=%PYTHON_BIN%"
) else (
    echo [WARNING] Python not found -- transcription will fail
)

:: Navigate to project root
cd /d "%~dp0"

:: Install frontend deps if needed
if not exist "frontend_v1\node_modules" (
    echo [BUILD] Installing frontend_v1 dependencies...
    cd frontend_v1 && call npm install && cd ..
)

:: Kill any existing process on port 8080
for /f "tokens=5" %%a in ('netstat -ano ^| findstr :8080 ^| findstr LISTENING 2^>nul') do (
    taskkill /F /PID %%a >nul 2>&1
)

:: Start Go backend (port 8080)
echo [START] Go backend on :8080
start /b "" cmd /c "go run ./cmd/web 2>&1"

:: Wait for backend
echo [WAIT] Waiting for backend...
:wait_backend
timeout /t 1 /nobreak >nul
curl -s http://127.0.0.1:8080/health >nul 2>&1
if errorlevel 1 goto wait_backend
echo [READY] Backend is up.

:: Start Go worker
echo [START] Go worker
start /b "" cmd /c "go run ./cmd/worker 2>&1"

:: Start Vite dev server (port 5173, proxies to 8080)
echo [START] Vite dev server on :5173
start /b "" cmd /c "cd frontend_v1 && npx vite --host 127.0.0.1 2>&1"

echo.
echo ============================================
echo.
echo   Frontend (dev):  http://127.0.0.1:5173/app-v1/
echo   Backend (prod):  http://localhost:8080/app-v1/
echo   Health:          http://localhost:8080/health
echo.
echo   Press Ctrl+C to stop all services
echo ============================================
echo.

:loop
timeout /t 60 /nobreak >nul
goto loop
