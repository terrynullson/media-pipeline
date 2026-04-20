#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "============================================"
echo "  Media Pipeline"
echo "============================================"
echo

require_command() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "[ERROR] Required command not found: $name" >&2
    exit 1
  fi
}

resolve_python() {
  if command -v python3 >/dev/null 2>&1; then
    printf '%s\n' "python3"
    return
  fi
  if command -v python >/dev/null 2>&1; then
    printf '%s\n' "python"
    return
  fi
  echo "[ERROR] Python 3 is required for the worker." >&2
  exit 1
}

database_config_present() {
  if [ -n "${DATABASE_URL:-}" ]; then
    return 0
  fi

  [ -n "${DB_HOST:-}" ] && [ -n "${DB_NAME:-}" ] && [ -n "${DB_USER:-}" ]
}

require_command go
require_command ffmpeg
require_command curl

PYTHON_BINARY="${PYTHON_BINARY:-$(resolve_python)}"
export PYTHON_BINARY

APP_PORT="${APP_PORT:-8080}"
export APP_PORT

if ! database_config_present; then
  echo "[ERROR] Database is not configured. Set DATABASE_URL or DB_HOST/DB_NAME/DB_USER before launch." >&2
  exit 1
fi

if [ ! -f "frontend_v1/dist/index.html" ]; then
  require_command npm
  echo "[BUILD] frontend_v1/dist is missing. Building frontend..."
  (
    cd frontend_v1
    [ -d node_modules ] || npm install
    npx vite build
  )
fi

echo "[BUILD] Building Go binaries..."
go build -o web ./cmd/web
go build -o worker ./cmd/worker

BACKEND_PID=""
WORKER_PID=""

cleanup() {
  local exit_code="${1:-0}"
  if [ -n "$WORKER_PID" ] && kill -0 "$WORKER_PID" >/dev/null 2>&1; then
    kill "$WORKER_PID" >/dev/null 2>&1 || true
  fi
  if [ -n "$BACKEND_PID" ] && kill -0 "$BACKEND_PID" >/dev/null 2>&1; then
    kill "$BACKEND_PID" >/dev/null 2>&1 || true
  fi
  wait >/dev/null 2>&1 || true
  exit "$exit_code"
}

trap 'cleanup 0' INT TERM

echo "[START] Web on :$APP_PORT"
./web &
BACKEND_PID=$!

echo "[WAIT] Waiting for http://127.0.0.1:$APP_PORT/health"
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:$APP_PORT/health" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! curl -fsS "http://127.0.0.1:$APP_PORT/health" >/dev/null 2>&1; then
  echo "[ERROR] Web did not become healthy within 60 seconds." >&2
  cleanup 1
fi

echo "[START] Worker"
./worker &
WORKER_PID=$!

echo
echo "  App:    http://localhost:$APP_PORT/app-v1"
echo "  Health: http://localhost:$APP_PORT/health"
echo "  Stop:   Ctrl+C"
echo

wait "$BACKEND_PID" "$WORKER_PID" || cleanup 1
