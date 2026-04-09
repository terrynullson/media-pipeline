#!/usr/bin/env bash
set -euo pipefail

echo "============================================"
echo "  Media Pipeline"
echo "============================================"
echo ""

# Navigate to script directory
cd "$(dirname "$0")"

# ── Check dependencies ──

check_cmd() {
  if command -v "$1" &>/dev/null; then
    echo "[OK] $1: $(command -v "$1")"
    return 0
  else
    echo "[MISSING] $1 not found in PATH"
    return 1
  fi
}

check_cmd go || { echo "[ERROR] Go is required. Install: https://go.dev/dl/"; exit 1; }
check_cmd ffmpeg || echo "[WARNING] ffmpeg not found -- audio extraction will fail"

# Find Python 3
PYTHON_BIN=""
for candidate in python3 python; do
  if command -v "$candidate" &>/dev/null; then
    ver=$("$candidate" --version 2>&1 | grep -oP '\d+\.\d+' | head -1)
    major=$(echo "$ver" | cut -d. -f1)
    if [ "$major" = "3" ]; then
      PYTHON_BIN="$candidate"
      echo "[OK] Python: $PYTHON_BIN ($ver)"
      break
    fi
  fi
done

if [ -z "$PYTHON_BIN" ]; then
  echo "[WARNING] Python 3 not found -- transcription will fail"
  echo "         Install Python 3 and faster-whisper"
else
  export PYTHON_BINARY="$PYTHON_BIN"
  # Check faster-whisper
  if "$PYTHON_BIN" -c "import faster_whisper" 2>/dev/null; then
    echo "[OK] faster-whisper installed"
  else
    echo "[WARNING] faster-whisper not installed: $PYTHON_BIN -m pip install faster-whisper"
  fi
fi

# Check Ollama (optional, for LLM summaries)
if command -v ollama &>/dev/null; then
  echo "[OK] Ollama available"
  export SUMMARY_PROVIDER="ollama"
else
  echo "[INFO] Ollama not found -- using simple summarizer (install: curl -fsSL https://ollama.com/install.sh | sh)"
fi

echo ""

# ── Build frontend if needed ──

if [ ! -f "frontend_v1/dist/index.html" ]; then
  echo "[BUILD] Building frontend_v1..."
  if ! command -v npm &>/dev/null; then
    echo "[ERROR] npm required to build frontend. Install Node.js."
    exit 1
  fi
  cd frontend_v1
  [ -d node_modules ] || npm install
  npx vite build
  cd ..
  echo "[BUILD] Frontend built."
  echo ""
fi

# ── Build Go binaries ──

echo "[BUILD] Building Go binaries..."
go build -o web ./cmd/web
go build -o worker ./cmd/worker
echo "[BUILD] Done."
echo ""

# ── Kill existing process on port ──

APP_PORT="${APP_PORT:-8080}"

if command -v lsof &>/dev/null; then
  existing_pid=$(lsof -ti :"$APP_PORT" 2>/dev/null || true)
  if [ -n "$existing_pid" ]; then
    echo "[CLEANUP] Killing existing process on :$APP_PORT (PID $existing_pid)"
    kill "$existing_pid" 2>/dev/null || true
    sleep 1
  fi
fi

# ── Trap for cleanup ──

BACKEND_PID=""
WORKER_PID=""

cleanup() {
  echo ""
  echo "[STOP] Shutting down..."
  [ -n "$BACKEND_PID" ] && kill "$BACKEND_PID" 2>/dev/null
  [ -n "$WORKER_PID" ] && kill "$WORKER_PID" 2>/dev/null
  wait 2>/dev/null
  echo "[STOP] All services stopped."
  exit 0
}

trap cleanup SIGINT SIGTERM

# ── Start backend ──

echo "[START] Go backend on :$APP_PORT"
./web &
BACKEND_PID=$!

# Wait for backend
echo "[WAIT] Waiting for backend..."
for i in $(seq 1 30); do
  if curl -s "http://127.0.0.1:$APP_PORT/health" >/dev/null 2>&1; then
    echo "[READY] Backend is up."
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "[ERROR] Backend did not start in 30 seconds"
    cleanup
  fi
  sleep 1
done

# ── Start worker ──

echo "[START] Go worker"
./worker &
WORKER_PID=$!

# ── Get network IP ──

LOCAL_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "???")

echo ""
echo "============================================"
echo ""
echo "  Local:   http://localhost:$APP_PORT/app-v1/"
echo "  Network: http://$LOCAL_IP:$APP_PORT/app-v1/"
echo "  Health:  http://localhost:$APP_PORT/health"
echo "  Logs:    data/logs/"
echo ""
echo "  Press Ctrl+C to stop all services"
echo "============================================"
echo ""

# Keep running
wait
