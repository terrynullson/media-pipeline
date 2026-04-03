# Media Pipeline

Minimal Go media intake service. Stage 3.1 adds real local transcription with `faster-whisper`, persisted transcription settings, and a simple server-rendered settings UI.

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

## Local Run On Windows

### What you need

- Go installed
- `ffmpeg` available in `PATH`
- Python 3 available through `py` or `python`

This repository now includes Windows-first helper scripts so local startup is one obvious command.

### One-command startup

PowerShell:

```powershell
.\scripts\dev_up.ps1
```

Batch:

```bat
scripts\dev_up.bat
```

What it does:
- ensures `data`, `data\uploads`, `data\audio`, and `data\logs` exist
- starts the web app in one PowerShell window
- starts the worker in another PowerShell window
- opens the browser to the main page unless you pass `-NoBrowser`
- writes logs to `data\logs\web.log` and `data\logs\worker.log`

Local URLs:
- Browser URL: `http://localhost:8080/`
- Health URL: `http://localhost:8080/health`

How to stop:
- press `Ctrl+C` in each PowerShell window
- or close the two windows opened by `dev_up.ps1`

### Start pieces manually

Web only:

```powershell
.\scripts\dev_web.ps1
```

Worker only:

```powershell
.\scripts\dev_worker.ps1
```

## Transcription Backend On Windows

### Install backend dependencies

PowerShell:

```powershell
.\scripts\install_transcription_backend.ps1
```

Batch:

```bat
scripts\install_transcription_backend.bat
```

What the installer does:
- detects whether `py` or `python` works on your machine
- creates a local `.venv` virtual environment
- upgrades `pip`
- installs packages from `scripts\requirements-transcription.txt`
- runs `scripts\transcribe.py --self-check`

The worker automatically prefers `.venv\Scripts\python.exe` when that virtual environment exists, so local Windows runs do not depend on the fragile `python` app alias.

### Backend details

The local transcription backend now uses `faster-whisper`.

Required Python packages are listed in `scripts\requirements-transcription.txt`.

Today that file installs:
- `faster-whisper`

The application now stores transcription settings in SQLite and lets you edit them in the web UI.

Default profile after first start:
- backend: `faster-whisper`
- model: `tiny`
- device: `cpu`
- compute type: `int8`
- beam size: `5`
- VAD: enabled

Supported UI settings:
- model: `tiny`, `base`, `small`
- device: `cpu`, `cuda`
- compute type for `cpu`: `int8`, `float32`
- compute type for `cuda`: `float16`, `int8_float16`
- language: blank means auto-detect
- beam size: `1` to `10`
- VAD: on or off

This is the safest default for a normal Windows laptop without GPU setup. It is practical for local testing, but CPU transcription can still be slow on longer files.

### Verify transcription backend

Self-check only:

```powershell
.\.venv\Scripts\python.exe .\scripts\transcribe.py --self-check --model-name tiny --device cpu --compute-type int8
```

Real transcription check on the sample WAV already in the repository:

```powershell
.\.venv\Scripts\python.exe .\scripts\transcribe.py --audio-path .\data\valid-upload.wav --model-name tiny --device cpu --compute-type int8 --beam-size 5 --vad-enabled true
```

The first real run may download the selected whisper model, so it can take noticeably longer than later runs.

### `PYTHON_BINARY` on Windows

If your machine does not resolve `python` correctly, set `PYTHON_BINARY` explicitly.

PowerShell example:

```powershell
$env:PYTHON_BINARY = ".\.venv\Scripts\python.exe"
go run ./cmd/worker
```

You can also set it permanently in your shell profile or your preferred `.env` workflow.

## Useful Commands

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
- `MAX_UPLOAD_SIZE_MB=500`
- `WORKER_POLL_INTERVAL_MS=2000`
- `FFMPEG_TIMEOUT_SEC=120`
- `TRANSCRIBE_TIMEOUT_SEC=300`

Uploaded files are stored in `UPLOAD_DIR/<UTC-date>/...`.
Extracted audio is stored in `AUDIO_DIR/<UTC-date>/media_<media_id>_<stored_name>.wav`.

## Worker Behavior

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
2. Successful audio extraction stores `media.extracted_audio_path` and enqueues a `transcribe` job with a snapshot of the effective transcription settings.
3. Successful transcription stores transcript text and timestamped segments.
4. Media status moves to `transcribed`.

## Transcription Settings

The main page now includes a small settings form.

It lets you:
- view the current default transcription profile
- edit and save backend, model, device, compute type, language, beam size, and VAD
- keep settings server-side validated before they are saved

The worker uses the settings snapshot from each `transcribe` job payload, so old jobs stay traceable even after you change the default profile later.

## Transcription Contract

The Python adapter calls `scripts/transcribe.py` as a subprocess.

Input arguments:
- `--audio-path` absolute path to extracted audio
- `--backend` backend name
- `--model-name` model name
- `--device` inference device
- `--compute-type` backend compute type
- `--language` optional language hint
- `--beam-size` decoding beam size
- `--vad-enabled` `true` or `false`
- `--self-check` to verify backend import and current configuration

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

If the transcription backend is not installed, the script exits with a useful stderr message and the worker saves that error on the failed `transcribe` job.

## Known Limitations

- The browser UI is intentionally simple: it helps with upload and basic inspection, not transcript browsing.
- The first `faster-whisper` run may download a model, so offline first-run transcription may fail until the model is cached.
- CPU transcription is appropriate for smoke tests and local development, but it is not fast for large files.
- `dev_up.ps1` starts separate PowerShell windows; it does not manage them as a background service manager.

## Docker

```bash
docker compose up --build
```

`docker compose` starts two services:
- `app` for the HTTP interface
- `worker` for polling, `extract_audio`, and `transcribe` execution

Both services share `/app/data`, so the web app and worker use the same SQLite database, uploaded files, extracted audio files, and transcript records.
