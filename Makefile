# ────────────────────────────────────────────────────────────────────────────
# media-pipeline — основной Makefile
# ────────────────────────────────────────────────────────────────────────────

.PHONY: run run-worker build fmt test verify-encoding docker-build compose-up

# ── Python-интерпретатор (кросс-платформенно) ────────────────────────────────
# Порядок: py (Windows Launcher) → python3 (Linux/macOS) → python (fallback).
# Переменную можно переопределить извне: make verify-encoding PYTHON=/usr/bin/python3
PYTHON ?= $(or \
  $(shell command -v py     2>/dev/null), \
  $(shell command -v python3 2>/dev/null), \
  $(shell command -v python  2>/dev/null))

# Сразу сообщаем, если Python не найден — лучше явная ошибка, чем
# загадочный «command not found» при запуске цели.
ifeq ($(strip $(PYTHON)),)
$(warning Python interpreter not found on PATH.)
$(warning Install Python 3 or set: make verify-encoding PYTHON=/path/to/python3)
PYTHON := python3   # дадим системе самой выдать внятную ошибку при запуске
endif

# ── Go-цели ──────────────────────────────────────────────────────────────────

run:
	go run ./cmd/web

run-worker:
	go run ./cmd/worker

build:
	go build ./...

fmt:
	go fmt ./...

test:
	go test ./...

# ── Качество / кодировки ─────────────────────────────────────────────────────

# Проверяет, что все текстовые файлы в src-каталогах:
#   - корректный UTF-8 без BOM
#   - не содержат mojibake (битой кириллицы)
# Падает с exit code 1 — подходит для CI.
# Использование:
#   make verify-encoding               # автодетект Python
#   make verify-encoding PYTHON=python3  # явный интерпретатор
verify-encoding:
	$(PYTHON) scripts/check-encoding.py

# ── Docker ───────────────────────────────────────────────────────────────────

docker-build:
	docker build -t media-pipeline:stage2 .

compose-up:
	docker compose up --build
