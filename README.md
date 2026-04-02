# Media Pipeline

Minimal Go media intake service. Stage 3 adds transcription processing for extracted audio through a clean Go `Transcriber` port and a Python subprocess adapter.

Current scope:
- `GET /`
- `POST /upload`
- `GET /health`
- SQLite persistence
- local filesystem storage
- startup migrations
- pending `extract_audio` job created after upload
- pending `transcribe` job created after successful audio extraction
- separate worker process for local media processing with `ffmpeg`
- transcript persistence in SQLite
- timestamped transcript segments in SQLite

## Quick start

Make sure `ffmpeg` and Python are available in `PATH`, or set `FFMPEG_BINARY` and `PYTHON_BINARY` explicitly.

On Windows, `PYTHON_BINARY=py` may work better than `python` if the default Python app alias is not enabled as a normal console executable.

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
- `PYTHON_BINARY=python`
- `TRANSCRIBE_SCRIPT=./scripts/transcribe.py`
- `TRANSCRIBE_LANGUAGE=`
- `MAX_UPLOAD_SIZE_MB=200`
- `WORKER_POLL_INTERVAL_MS=2000`
- `FFMPEG_TIMEOUT_SEC=120`
- `TRANSCRIBE_TIMEOUT_SEC=300`

Uploaded files are stored in `UPLOAD_DIR/<UTC-date>/...`.
Extracted audio is stored in `AUDIO_DIR/<UTC-date>/media_<media_id>_<stored_name>.wav`.

## Worker behavior

The worker:
- polls SQLite on a small interval
- processes one job at a time
- claims `pending` jobs of type `extract_audio`, then `transcribe`
- marks jobs as `running`, then `done` or `failed`
- stores useful `error_message` values on failure
- requeues interrupted `running` jobs back to `pending` on startup
- runs `ffmpeg` with a timeout
- runs the Python transcription script with a timeout
- persists transcript header text and transcript segments
- keeps processing later jobs even if one transcription fails

Job flow is explicit:

1. Upload creates an `extract_audio` job.
2. Successful audio extraction stores `media.extracted_audio_path` and enqueues a `transcribe` job.
3. Successful transcription stores:
   - `media.transcript_text`
   - one row in `transcripts`
   - many rows in `transcript_segments`
4. Media status moves to `transcribed`.

## Transcription contract

The Python adapter calls `scripts/transcribe.py` as a subprocess.

Input arguments:
- `--audio-path` absolute path to extracted audio
- `--language` optional language hint

Expected JSON on stdout:

```json
{
  "full_text": "hello world",
  "segments": [
    {
      "start_sec": 0.0,
      "end_sec": 1.2,
      "text": "hello",
      "confidence": 0.95
    }
  ]
}
```

The bundled script is intentionally minimal. It is structured so it can later use `faster-whisper` or be replaced with another backend without changing the Go worker contract.

If no transcription backend is installed, the script exits with a useful stderr message and the worker saves that error on the failed `transcribe` job.

## Docker

```bash
docker compose up --build
```

`docker compose` starts two services:
- `app` for the HTTP interface
- `worker` for polling, `extract_audio`, and `transcribe` execution

Both services share `/app/data`, so the web app and worker use the same SQLite database, uploaded files, extracted audio files, and transcript records.
