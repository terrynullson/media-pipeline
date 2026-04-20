# Media Pipeline

`media-pipeline` принимает медиафайлы, складывает их в файловое хранилище, ставит pipeline-задачи в PostgreSQL и обрабатывает их отдельным worker-процессом: готовит browser-safe preview, извлекает аудио, делает транскрипцию, ищет триггеры, сохраняет скриншоты и собирает summary.

Основной и подтверждённый сценарий запуска проекта: `service/binary mode` без обязательного Docker.

## Что реально нужно для запуска

Обязательно:

- Go 1.24+
- PostgreSQL 16+ или 17+
- `ffmpeg`
- Python 3.10+
- рабочий backend для `scripts/transcribe.py`

Нужно только если `frontend_v1/dist` отсутствует или вы меняли фронтенд:

- Node.js + npm

Опционально:

- Ollama, если нужен `SUMMARY_PROVIDER=ollama`

## Обязательные переменные окружения

Минимум для `web` и `worker`:

- `DATABASE_URL`

Чаще всего также настраивают:

- `APP_PORT` по умолчанию `8080`
- `FFMPEG_BINARY` по умолчанию `ffmpeg`
- `PYTHON_BINARY` по умолчанию `python` или `py`
- `TRANSCRIBE_SCRIPT` по умолчанию `./scripts/transcribe.py`
- `TEST_DATABASE_URL` для Postgres-backed тестов

Актуальный пример есть в [.env.example](/E:/speech-to-text/media-pipeline_1.0/.env.example).

## Быстрый запуск на Windows

1. Скопируйте `.env.example` в `.env` и при необходимости поправьте значения.
2. Запустите:

```powershell
start.bat
```

Что делает `start.bat`:

- загружает `.env`
- поднимает локальный PostgreSQL-кластер на `127.0.0.1:55432`, если он ещё не запущен
- создаёт основную и тестовую БД, если их нет
- запускает `web`
- ждёт успешный ответ `GET /health`
- запускает `worker`

Открывать в браузере:

- приложение: `http://127.0.0.1:8080/app-v1`
- health: `http://127.0.0.1:8080/health`

Остановка:

```powershell
stop.bat
```

`stop.bat` останавливает:

- PowerShell-процесс `web`
- PowerShell-процесс `worker`
- локальный PostgreSQL-кластер проекта

## Ручной запуск

Если нужен ручной режим без `start.bat`:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\dev_db.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\dev_web.ps1
# дождаться /health
powershell -ExecutionPolicy Bypass -File .\scripts\dev_worker.ps1
```

Порядок запуска важен:

1. сначала `web`
2. `web` открывает БД и применяет миграции
3. только после успешного `/health` запускается `worker`

## Linux / macOS

Поддерживается `start.sh`, но в ходе практической проверки подтверждён именно Windows-сценарий с `start.bat` и локальным PostgreSQL через `scripts/dev_db.ps1`.

## Что было реально проверено

Подтверждено на локальном запуске:

- `go build ./...`
- `go test ./...`
- сборка фронтенда через `npm run build`
- запуск локального PostgreSQL на `127.0.0.1:55432`
- успешный старт `web`
- успешный старт `worker`
- ответ `GET /health` со статусом `200`
- ответ `GET /api/media` со статусом `200`
- ответ `GET /api/worker/status` со статусом `200`
- корректная работа `stop.bat`

Дополнительно подтверждено:

- во время длинного preview-этапа `worker status` больше не показывает ложное состояние `likelyAlive=false`
- пользовательские строки во фронтенде и в карточке preview больше не разваливаются на `????`

## Архитектура

```text
HTTP client
  -> cmd/web
     -> app use cases
        -> domain ports
           -> infra repositories/adapters
              -> PostgreSQL / filesystem / ffmpeg / Python

cmd/worker
  -> app/worker runner
     -> repositories + infra adapters
```

Ключевые принципы:

- PostgreSQL используется как основной runtime storage
- домен и app-слой не зависят от деталей PostgreSQL
- тяжёлая обработка медиа не выполняется в HTTP handlers
- миграции применяются на старте `web`
- `worker` работает отдельно и не должен запускаться раньше готового `web`
