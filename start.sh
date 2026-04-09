#!/usr/bin/env bash
set -euo pipefail

echo "============================================"
echo "  Media Pipeline"
echo "============================================"
echo ""

# Navigate to script directory
cd "$(dirname "$0")"

# ── Helper: ask user y/n ──

ask_install() {
  local name="$1"
  read -r -p "[?] $name not found. Install? [y/N] " answer
  case "$answer" in
    [yY]|[yY][eE][sS]) return 0 ;;
    *) return 1 ;;
  esac
}

# Detect package manager
PKG=""
if command -v apt-get &>/dev/null; then
  PKG="apt"
elif command -v dnf &>/dev/null; then
  PKG="dnf"
elif command -v yum &>/dev/null; then
  PKG="yum"
elif command -v pacman &>/dev/null; then
  PKG="pacman"
fi

pkg_install() {
  echo "[INSTALL] Installing $1..."
  case "$PKG" in
    apt)    sudo apt-get update -qq && sudo apt-get install -y "$@" ;;
    dnf)    sudo dnf install -y "$@" ;;
    yum)    sudo yum install -y "$@" ;;
    pacman) sudo pacman -S --noconfirm "$@" ;;
    *)      echo "[ERROR] Unknown package manager. Install manually: $*"; return 1 ;;
  esac
}

# ── Check & install dependencies ──

# Go
if command -v go &>/dev/null; then
  echo "[OK] go: $(go version | awk '{print $3}')"
else
  if ask_install "Go"; then
    if [ "$PKG" = "apt" ]; then
      # Use official method for latest Go on Debian/Ubuntu
      echo "[INSTALL] Downloading Go 1.22..."
      curl -fsSL "https://go.dev/dl/go1.22.5.linux-amd64.tar.gz" -o /tmp/go.tar.gz
      sudo rm -rf /usr/local/go
      sudo tar -C /usr/local -xzf /tmp/go.tar.gz
      rm /tmp/go.tar.gz
      export PATH="/usr/local/go/bin:$PATH"
      echo 'export PATH="/usr/local/go/bin:$PATH"' >> ~/.bashrc
    else
      pkg_install golang
    fi
    echo "[OK] go: $(go version | awk '{print $3}')"
  else
    echo "[ERROR] Go is required. Install: https://go.dev/dl/"
    exit 1
  fi
fi

# ffmpeg
if command -v ffmpeg &>/dev/null; then
  echo "[OK] ffmpeg: $(ffmpeg -version 2>&1 | head -1 | awk '{print $3}')"
else
  if ask_install "ffmpeg"; then
    pkg_install ffmpeg
    echo "[OK] ffmpeg installed"
  else
    echo "[WARNING] ffmpeg not found -- audio extraction will fail"
  fi
fi

# Python 3
PYTHON_BIN=""
for candidate in python3 python; do
  if command -v "$candidate" &>/dev/null; then
    ver=$("$candidate" --version 2>&1 | grep -oP '\d+\.\d+' | head -1)
    major=$(echo "$ver" | cut -d. -f1)
    if [ "$major" = "3" ]; then
      PYTHON_BIN="$candidate"
      break
    fi
  fi
done

if [ -n "$PYTHON_BIN" ]; then
  echo "[OK] Python: $PYTHON_BIN ($("$PYTHON_BIN" --version 2>&1))"
else
  if ask_install "Python 3"; then
    pkg_install python3 python3-pip python3-venv
    PYTHON_BIN="python3"
    echo "[OK] Python: $PYTHON_BIN"
  else
    echo "[WARNING] Python 3 not found -- transcription will fail"
  fi
fi

if [ -n "$PYTHON_BIN" ]; then
  export PYTHON_BINARY="$PYTHON_BIN"
  # Check & install faster-whisper
  if "$PYTHON_BIN" -c "import faster_whisper" 2>/dev/null; then
    echo "[OK] faster-whisper installed"
  else
    if ask_install "faster-whisper (Python package)"; then
      "$PYTHON_BIN" -m pip install --user faster-whisper
      echo "[OK] faster-whisper installed"
    else
      echo "[WARNING] faster-whisper not installed -- transcription will fail"
    fi
  fi
fi

# Node.js + npm (for frontend build)
if command -v npm &>/dev/null; then
  echo "[OK] npm: $(npm --version)"
else
  if [ ! -f "frontend_v1/dist/index.html" ]; then
    if ask_install "Node.js + npm (needed to build frontend)"; then
      if [ "$PKG" = "apt" ]; then
        # Install via NodeSource for a recent version
        curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
        sudo apt-get install -y nodejs
      else
        pkg_install nodejs npm
      fi
      echo "[OK] npm: $(npm --version)"
    else
      echo "[WARNING] npm not found -- cannot build frontend"
    fi
  fi
fi

# curl (needed for health check)
if ! command -v curl &>/dev/null; then
  if ask_install "curl"; then
    pkg_install curl
  fi
fi

# Ollama (optional, for LLM summaries)
if command -v ollama &>/dev/null; then
  echo "[OK] Ollama available"
  export SUMMARY_PROVIDER="ollama"
  # Check if model is pulled
  if ! ollama list 2>/dev/null | grep -q "phi3:mini"; then
    read -r -p "[?] Ollama model phi3:mini not found. Pull it? [y/N] " answer
    case "$answer" in
      [yY]|[yY][eE][sS]) ollama pull phi3:mini ;;
    esac
  fi
else
  read -r -p "[?] Ollama not found (needed for AI summaries, optional). Install? [y/N] " answer
  case "$answer" in
    [yY]|[yY][eE][sS])
      curl -fsSL https://ollama.com/install.sh | sh
      echo "[OK] Ollama installed"
      export SUMMARY_PROVIDER="ollama"
      echo "[INSTALL] Pulling phi3:mini model..."
      ollama pull phi3:mini
      ;;
    *)
      echo "[INFO] Using simple summarizer (no AI)"
      ;;
  esac
fi

echo ""
echo "[OK] All dependencies checked."
echo ""

# ── Build frontend if needed ──

if [ ! -f "frontend_v1/dist/index.html" ]; then
  if command -v npm &>/dev/null; then
    echo "[BUILD] Building frontend_v1..."
    cd frontend_v1
    [ -d node_modules ] || npm install
    npx vite build
    cd ..
    echo "[BUILD] Frontend built."
    echo ""
  else
    echo "[WARNING] Cannot build frontend -- npm not available"
  fi
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
