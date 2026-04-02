# Media Pipeline

Минимальный media intake сервис на Go. Stage 2 добавляет отдельный worker, который забирает pending job `extract_audio` из SQLite и извлекает WAV-аудио через `ffmpeg`.

Текущий scope:
- `GET /`
- `POST /upload`
- `GET /health`
- SQLite persistence
- local filesystem storage
- startup migrations
- pending job `extract_audio` после загрузки файла
- отдельный worker process для обработки media через `ffmpeg`

## Quick start

Нужно, чтобы `ffmpeg` был доступен в `PATH` или был указан через `FFMPEG_BINARY`.

1. Запустите web-приложение.
2. В отдельном терминале запустите worker.

```bash
make run
make run-worker
```

По умолчанию web доступен на `http://localhost:8080`.

## Useful commands

```bash
make fmt
make build
make test
make run
make run-worker
```

## Environment

Значения по умолчанию берутся из `.env.example`.

- `APP_PORT=8080`
- `DB_PATH=./data/app.db`
- `UPLOAD_DIR=./data/uploads`
- `AUDIO_DIR=./data/audio`
- `FFMPEG_BINARY=ffmpeg`
- `MAX_UPLOAD_SIZE_MB=200`
- `WORKER_POLL_INTERVAL_MS=2000`
- `FFMPEG_TIMEOUT_SEC=120`

Загруженные файлы сохраняются в `UPLOAD_DIR/<UTC-date>/...`.
Извлечённое аудио сохраняется в `AUDIO_DIR/<UTC-date>/media_<media_id>_<stored_name>.wav`.

## Docker

```bash
docker compose up --build
```

`docker compose` поднимает два сервиса:
- `app` для HTTP-интерфейса
- `worker` для polling и выполнения `extract_audio`

Оба сервиса используют общий том `/app/data`, поэтому web и worker видят одну SQLite-базу, загруженные файлы и результирующее аудио.
