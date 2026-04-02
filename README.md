# Media Pipeline

Minimal Go media intake service. Stage 2 adds a separate worker that polls SQLite for pending `extract_audio` jobs and extracts WAV audio with `ffmpeg`.

Current scope:
- `GET /`
- `POST /upload`
- `GET /health`
- SQLite persistence
- local filesystem storage
- startup migrations
- pending `extract_audio` job created after upload
- separate worker process for local media processing with `ffmpeg`

## Quick start

Make sure `ffmpeg` is available in `PATH` or set `FFMPEG_BINARY` explicitly.

1. Start the web app.
2. Start the worker in a separate terminal.

```bash
make run
make run-worker
```

The web app listens on `http://localhost:8080` by default.

## Useful commands

```bash
make fmt
make build
make test
make run
make run-worker
```

## Environment

Default values come from `.env.example`.

- `APP_PORT=8080`
- `DB_PATH=./data/app.db`
- `UPLOAD_DIR=./data/uploads`
- `AUDIO_DIR=./data/audio`
- `FFMPEG_BINARY=ffmpeg`
- `MAX_UPLOAD_SIZE_MB=200`
- `WORKER_POLL_INTERVAL_MS=2000`
- `FFMPEG_TIMEOUT_SEC=120`

Uploaded files are stored in `UPLOAD_DIR/<UTC-date>/...`.
Extracted audio is stored in `AUDIO_DIR/<UTC-date>/media_<media_id>_<stored_name>.wav`.

## Worker behavior

The worker:
- polls SQLite on a small interval
- processes one job at a time
- claims only `pending` jobs of type `extract_audio`
- marks jobs as `running`, then `done` or `failed`
- stores useful `error_message` values on failure
- requeues interrupted `running` jobs back to `pending` on startup
- runs `ffmpeg` with a timeout

## Docker

```bash
docker compose up --build
```

`docker compose` starts two services:
- `app` for the HTTP interface
- `worker` for polling and `extract_audio` execution

Both services share `/app/data`, so the web app and worker use the same SQLite database, uploaded files, and extracted audio files.
