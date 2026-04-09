# Media Pipeline

Upload video/audio, extract audio, transcribe speech, detect trigger keywords, capture screenshots at trigger points, generate summaries.

**Stack:** Go backend + worker, React frontend (Vite + TypeScript), SQLite, ffmpeg, faster-whisper (Python), Ollama (optional).

## Features

- Drag & drop upload (multiple files, up to 1 GB each)
- Automatic pipeline: extract audio -> transcribe -> analyze triggers -> screenshots -> summary
- Real-time progress tracking on the home page with expandable pipeline steps
- Transcript viewer with segment sync to video playback
- Full-text search across transcripts
- Trigger keyword matching (exact / contains) with screenshots
- AI summary via Ollama (Phi-3 Mini) or simple local algorithm
- Settings UI: whisper model, device, compute type, language, beam size, VAD, trigger rules
- i18n: Russian and English interface
- Light and dark theme with smooth transitions
- Responsive design

## Quick Start

### Windows

```bat
start.bat
```

### Linux / macOS

```bash
chmod +x start.sh
./start.sh
```

The start script will:
- Check all dependencies (Go, ffmpeg, Python 3, faster-whisper, Node.js, Ollama)
- Offer to install any missing dependency interactively (`[y/N]` prompts)
- Build the frontend (if not already built)
- Build Go binaries
- Start the backend and worker
- Show local and network URLs

Open `http://localhost:8080/app-v1/` in your browser.

### Dependencies

| Dependency | Required | Purpose |
|---|---|---|
| Go 1.21+ | Yes | Backend + worker |
| ffmpeg | Yes | Audio extraction, previews, screenshots |
| Python 3.10+ | Yes | Transcription |
| faster-whisper | Yes | Speech-to-text engine |
| Node.js + npm | Once | Frontend build |
| Ollama | Optional | AI-powered summaries |

### Manual install (Debian/Ubuntu)

```bash
sudo apt install golang ffmpeg python3 python3-pip nodejs npm curl
pip3 install faster-whisper

# Optional: AI summaries
curl -fsSL https://ollama.com/install.sh | sh
ollama pull phi3:mini
```

## Architecture

```
Browser -> Go backend (:8080) -> SQLite
                                    ^
                              Go worker (polls jobs)
                                    |
                      ffmpeg / Python transcribe / Ollama
```

**Backend** (`cmd/web`): HTTP API, file upload, static frontend serving, settings management.

**Worker** (`cmd/worker`): Processes pipeline jobs sequentially. Later stages have priority so each file completes fully before the next one starts.

**Frontend** (`frontend_v1`): React 18 SPA served at `/app-v1`. Built with Vite, uses lucide-react for icons.

### Pipeline stages

1. **Extract audio** -- ffmpeg converts uploaded media to WAV
2. **Transcribe** -- faster-whisper produces timestamped segments
3. **Analyze triggers** -- matches keyword rules against transcript
4. **Extract screenshots** -- ffmpeg captures frames at trigger timestamps
5. **Prepare preview** -- ffmpeg creates browser-compatible preview video
6. **Generate summary** -- Ollama LLM or simple local algorithm

## Configuration

Environment variables (defaults in parentheses):

| Variable | Default | Description |
|---|---|---|
| `APP_PORT` | `8080` | HTTP server port |
| `DB_PATH` | `./data/app.db` | SQLite database path |
| `UPLOAD_DIR` | `./data/uploads` | Uploaded files storage |
| `AUDIO_DIR` | `./data/audio` | Extracted audio storage |
| `PREVIEW_DIR` | `./data/previews` | Preview videos storage |
| `SCREENSHOTS_DIR` | `./data/screenshots` | Trigger screenshots storage |
| `FFMPEG_BINARY` | `ffmpeg` | Path to ffmpeg |
| `PYTHON_BINARY` | `python` | Path to Python 3 |
| `TRANSCRIBE_SCRIPT` | `./scripts/transcribe.py` | Transcription script |
| `TRANSCRIBE_LANGUAGE` | *(auto-detect)* | Force transcription language |
| `MAX_UPLOAD_SIZE_MB` | `1024` | Max upload size in MB |
| `SUMMARY_PROVIDER` | `simple` | Summary engine: `simple` or `ollama` |
| `OLLAMA_URL` | `http://127.0.0.1:11434` | Ollama API endpoint |
| `OLLAMA_MODEL` | `phi3:mini` | Ollama model for summaries |

## Transcription Settings

Configurable via the Settings drawer in the UI:

| Setting | Options |
|---|---|
| Model | `tiny`, `base`, `small`, `medium`, `large` |
| Device | `cpu`, `cuda` |
| Compute type (CPU) | `int8`, `float32` |
| Compute type (CUDA) | `float16`, `int8_float16` |
| Language | blank = auto-detect |
| Beam size | 1--10 |
| VAD filter | on / off |

Model recommendations:
- **tiny/base** -- fast, lower quality, good for testing
- **small** -- balanced, recommended for most use cases
- **medium** -- better quality, ~5 GB VRAM or slow on CPU
- **large** -- best quality, ~10 GB VRAM, very slow on CPU

## Trigger Rules

Managed via the Settings drawer. Each rule has:
- **Name** -- display label
- **Category** -- grouping tag
- **Pattern** -- text to match in transcript
- **Match mode** -- `contains` (substring) or `exact` (whole segment)

When a trigger matches, the worker captures a screenshot from the video at that timestamp.

## Network Access

After starting, the script shows the network URL:

```
  Local:   http://localhost:8080/app-v1/
  Network: http://192.168.1.50:8080/app-v1/
```

Any device on the same network can access the app via the Network URL. For internet access, set up nginx as a reverse proxy with HTTPS.

## Project Structure

```
cmd/
  web/           -- HTTP server entry point
  worker/        -- Background job processor entry point
internal/
  app/           -- Application use cases
  domain/        -- Domain models (job, media, transcript, trigger)
  infra/         -- Infrastructure (DB, config, ffmpeg, transcription, summary)
  transport/     -- HTTP handlers and router
frontend_v1/     -- React frontend (Vite + TypeScript)
scripts/         -- Python transcription script
data/            -- Runtime data (DB, uploads, audio, logs)
start.bat        -- Windows launch script
start.sh         -- Linux/macOS launch script
```

## License

Private project.
