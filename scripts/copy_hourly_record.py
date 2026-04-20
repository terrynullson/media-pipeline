from __future__ import annotations

import logging
import shutil
import sys
import time
from datetime import datetime, timedelta
from pathlib import Path


# =========================
# НАСТРОЙКИ
# =========================

# УКАЖИ РЕАЛЬНЫЙ ПУТЬ К СЕТЕВОЙ ПАПКЕ
# Примеры:
# SOURCE_ROOT = Path(r"\\192.168.1.50\records")
# SOURCE_ROOT = Path(r"\\NAS\air_records")
# SOURCE_ROOT = Path(r"Z:\")
SOURCE_ROOT = Path(r"\\172.17.13.2\Police Record")

# Куда копировать файлы
DEST_ROOT = Path(r"C:\media-pipline\media-pipeline-elated-lalande\data\auto_uploads")

# Куда писать лог
LOG_DIR = Path(r"C:\media-pipline\media-pipeline-elated-lalande\data\logs")
LOG_FILE = LOG_DIR / "copy_hourly_record.log"

# Префикс файла
FILE_PREFIX = "Recorder_1_"

# Расширение файла
FILE_EXTENSION = ".mp4"


def setup_logging() -> None:
    LOG_DIR.mkdir(parents=True, exist_ok=True)

    formatter = logging.Formatter("%(asctime)s | %(levelname)s | %(message)s")

    logger = logging.getLogger()
    logger.setLevel(logging.INFO)
    logger.handlers.clear()

    file_handler = logging.FileHandler(LOG_FILE, encoding="utf-8")
    file_handler.setFormatter(formatter)

    stream_handler = logging.StreamHandler(sys.stdout)
    stream_handler.setFormatter(formatter)

    logger.addHandler(file_handler)
    logger.addHandler(stream_handler)


def now_local() -> datetime:
    """
    Берём локальное время Windows.
    Важно: на компьютере должен быть выставлен нужный часовой пояс.
    """
    return datetime.now()


def build_expected_paths(target_dt: datetime) -> tuple[Path, Path]:
    """
    Пример:
    папка: 26_04_16
    файл: Recorder_1_26.04.16_10.00.00.00.mp4
    """
    day_folder_name = target_dt.strftime("%y_%m_%d")
    file_name = f"{FILE_PREFIX}{target_dt.strftime('%y.%m.%d_%H.00.00.00')}{FILE_EXTENSION}"

    source_file = SOURCE_ROOT / day_folder_name / file_name
    dest_file = DEST_ROOT / day_folder_name / file_name

    return source_file, dest_file


def seconds_until_next_run(now: datetime) -> int:
    """
    Сколько секунд до ближайшего HH:10.
    Примеры:
    10:22 -> ждать до 11:10
    10:05 -> ждать до 10:10
    """
    next_run = now.replace(minute=10, second=0, microsecond=0)

    if now.minute >= 10:
        next_run += timedelta(hours=1)

    return max(1, int((next_run - now).total_seconds()))


def check_source_root() -> bool:
    try:
        exists = SOURCE_ROOT.exists()
        if exists:
            logging.info("Подключение к сетевой папке есть: %s", SOURCE_ROOT)
            return True

        logging.error("Сетевая папка недоступна или не существует: %s", SOURCE_ROOT)
        return False
    except Exception as exc:
        logging.exception("Ошибка при проверке сетевой папки %s: %s", SOURCE_ROOT, exc)
        return False


def copy_if_needed(source_file: Path, dest_file: Path) -> None:
    if not source_file.exists():
        logging.warning("Ожидаемый файл пока не найден: %s", source_file)
        return

    dest_file.parent.mkdir(parents=True, exist_ok=True)

    if dest_file.exists():
        try:
            source_size = source_file.stat().st_size
            dest_size = dest_file.stat().st_size
        except Exception as exc:
            logging.exception("Не удалось сравнить размеры файлов: %s", exc)
            return

        if source_size == dest_size:
            logging.info("Файл уже существует и размер совпадает, пропускаю: %s", dest_file)
            return

        logging.warning(
            "Файл уже существует, но размер отличается. Перекопирую. source=%s bytes, dest=%s bytes",
            source_size,
            dest_size,
        )

    shutil.copy2(source_file, dest_file)
    logging.info("Файл успешно скопирован: %s -> %s", source_file, dest_file)


def run_check() -> None:
    now_dt = now_local()
    target_dt = (now_dt - timedelta(hours=1)).replace(minute=0, second=0, microsecond=0)

    logging.info("Запуск проверки. Текущее локальное время: %s", now_dt.strftime("%Y-%m-%d %H:%M:%S"))
    logging.info("Нужно проверить файл за предыдущий час: %s", target_dt.strftime("%Y-%m-%d %H:%M:%S"))

    if not check_source_root():
        logging.error("Проверка отменена: нет доступа к сетевой папке")
        return

    source_file, dest_file = build_expected_paths(target_dt)

    logging.info("Ожидаемый исходный файл: %s", source_file)
    logging.info("Путь назначения: %s", dest_file)

    copy_if_needed(source_file, dest_file)


def run_loop() -> None:
    logging.info("Процесс запущен")
    logging.info("Лог-файл: %s", LOG_FILE)
    logging.info("Сетевая папка: %s", SOURCE_ROOT)
    logging.info("Папка назначения: %s", DEST_ROOT)

    while True:
        now_dt = now_local()
        wait_seconds = seconds_until_next_run(now_dt)
        next_run = now_dt + timedelta(seconds=wait_seconds)

        logging.info(
            "Процесс активен. Текущее локальное время: %s | Ожидаю %s сек. до %s для начала сканирования",
            now_dt.strftime("%Y-%m-%d %H:%M:%S"),
            wait_seconds,
            next_run.strftime("%Y-%m-%d %H:%M:%S"),
        )

        time.sleep(wait_seconds)

        try:
            run_check()
        except Exception as exc:
            logging.exception("Непредвиденная ошибка в основном цикле: %s", exc)

        logging.info("Цикл завершён, ожидаю следующий запуск")


if __name__ == "__main__":
    setup_logging()

    try:
        run_loop()
    except KeyboardInterrupt:
        logging.info("Скрипт остановлен вручную")
        print("\nСкрипт остановлен вручную.")
    except Exception as exc:
        logging.exception("Критическая ошибка при запуске: %s", exc)
        print(f"\nКритическая ошибка: {exc}")
        input("Нажми Enter, чтобы закрыть окно...")