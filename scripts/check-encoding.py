#!/usr/bin/env python3
"""
check-encoding.py — защитная проверка кодировок проекта.

Что проверяет:
  1. Все текстовые файлы в указанных каталогах декодируются как чистый UTF-8.
  2. Ни один файл не начинается с BOM (U+FEFF / 0xEF BB BF) — мы не используем BOM.
  3. Ни один файл не содержит явного «мусора» кодировки (mojibake):
     типичный паттерн — последовательности из 2+ символов вида «Р·», «СЃ», которые
     появляются, когда UTF-8 кириллица была ошибочно прочитана как Windows-1251.

Запуск:
  python scripts/check-encoding.py
  python scripts/check-encoding.py --fix   (исправляет mojibake на месте, --fix экспериментальный)

Выход:
  0 — всё чисто
  1 — найдены проблемы (CI должен считать это ошибкой)
"""

import argparse
import os
import re
import sys

# ---------------------------------------------------------------------------
# Каталоги и расширения, которые проверяем
# ---------------------------------------------------------------------------

CHECK_DIRS = [
    "internal",
    "cmd",
    "frontend_v1/src",
    "web",
    "scripts",
]

TEXT_EXTENSIONS = {
    ".go", ".html", ".md", ".sql", ".json",
    ".ts", ".tsx", ".js", ".css",
    ".yml", ".yaml", ".sh", ".py",
}

SKIP_DIRS = {".git", "node_modules", "dist", ".claude", "vendor", "__pycache__"}

# ---------------------------------------------------------------------------
# Mojibake detection
# ---------------------------------------------------------------------------
# Строим полный набор символов, которые появляются при ошибочном чтении
# UTF-8 байт как Windows-1251. Каждый байт 0x00-0xFF через cp1251 даёт
# конкретный Unicode-символ; байты 0x80-0xFF дают не-ASCII символы.
_MOJIBAKE_CHARS: set[str] = set()
for _b in range(0x00, 0x100):
    try:
        _c = bytes([_b]).decode("cp1251")
        if ord(_c) > 0x7F:
            _MOJIBAKE_CHARS.add(_c)
    except (UnicodeDecodeError, ValueError):
        pass

# Паттерн: 2+ символа из «мусорного» набора подряд — это потенциальный mojibake.
# Мы требуем >=2, чтобы не ловить легитимные одиночные спецсимволы (©, °, ±…).
_MOJIBAKE_PAT = re.compile(
    r"[" + re.escape("".join(sorted(_MOJIBAKE_CHARS))) + r"]{2,}"
)


def _looks_like_mojibake(run: str) -> bool:
    """
    Проверяем, является ли run настоящим mojibake, а не легитимным текстом.
    Критерий: строку можно cp1251-закодировать и utf-8-раскодировать,
    и в результате появляется кириллица.
    """
    try:
        decoded = run.encode("cp1251", errors="strict").decode("utf-8", errors="strict")
        return bool(re.search(r"[а-яА-ЯёЁ]", decoded))
    except (UnicodeEncodeError, UnicodeDecodeError):
        return False


# ---------------------------------------------------------------------------
# Fix logic (используется только с --fix)
# ---------------------------------------------------------------------------

def _fix_run(run: str) -> str:
    """Пытается исправить один mojibake-прогон."""
    # Пробуем всю строку целиком
    try:
        decoded = run.encode("cp1251", errors="strict").decode("utf-8", errors="strict")
        if re.search(r"[а-яА-ЯёЁ]", decoded):
            return decoded
    except (UnicodeEncodeError, UnicodeDecodeError):
        pass

    # Иначе: берём максимальный prefix из символов mojibake-набора и декодируем его
    prefix = list(run)
    while prefix:
        candidate = [c for c in prefix if c in _MOJIBAKE_CHARS]
        if not candidate or len(candidate) != len(prefix):
            # Строка содержит «чистые» символы — берём только mojibake-prefix
            prefix_only = []
            for ch in run:
                if ch in _MOJIBAKE_CHARS:
                    prefix_only.append(ch)
                else:
                    break
            s = "".join(prefix_only)
            try:
                decoded = s.encode("cp1251", errors="strict").decode("utf-8", errors="strict")
                if re.search(r"[а-яА-ЯёЁ]", decoded):
                    return decoded + run[len(prefix_only):]
            except (UnicodeEncodeError, UnicodeDecodeError):
                pass
            break
        s = "".join(prefix)
        try:
            decoded = s.encode("cp1251", errors="strict").decode("utf-8", errors="strict")
            if re.search(r"[а-яА-ЯёЁ]", decoded):
                return decoded + run[len(prefix):]
        except (UnicodeEncodeError, UnicodeDecodeError):
            pass
        prefix.pop()

    return run  # не смогли исправить — возвращаем как есть


_NONASCII_PAT = re.compile(r"[^\x00-\x7F]+")


def fix_text(text: str) -> str:
    """Исправляет все mojibake-прогоны в тексте."""
    def _fixer(m: re.Match) -> str:
        run = m.group()
        if _looks_like_mojibake(run) or _MOJIBAKE_PAT.search(run):
            return _fix_run(run)
        return run
    return _NONASCII_PAT.sub(_fixer, text)


# ---------------------------------------------------------------------------
# File walker
# ---------------------------------------------------------------------------

def iter_files(check_dirs: list[str]):
    for base in check_dirs:
        if not os.path.exists(base):
            continue
        for root, dirs, files in os.walk(base):
            dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
            for fname in files:
                _, ext = os.path.splitext(fname)
                if ext.lower() in TEXT_EXTENSIONS:
                    yield os.path.join(root, fname)


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main() -> int:
    parser = argparse.ArgumentParser(description="Check source files for encoding problems.")
    parser.add_argument("--fix", action="store_true", help="Auto-fix detected mojibake (experimental).")
    parser.add_argument("dirs", nargs="*", default=CHECK_DIRS, help="Directories to scan.")
    args = parser.parse_args()

    errors: list[str] = []

    for path in iter_files(args.dirs):
        try:
            with open(path, "rb") as fh:
                raw = fh.read()
        except OSError as e:
            errors.append(f"{path}: cannot read: {e}")
            continue

        # 1. BOM check
        if raw.startswith(b"\xef\xbb\xbf"):
            errors.append(f"{path}: starts with UTF-8 BOM — remove it")

        # 2. UTF-8 validity
        try:
            text = raw.decode("utf-8")
        except UnicodeDecodeError as e:
            errors.append(f"{path}: not valid UTF-8: {e}")
            continue

        # 3. Replacement character (U+FFFD) — sign of lossy conversion
        if "\ufffd" in text:
            count = text.count("\ufffd")
            errors.append(f"{path}: contains {count} replacement character(s) U+FFFD")

        # 4. Mojibake detection
        moji_hits = []
        for m in _MOJIBAKE_PAT.finditer(text):
            if _looks_like_mojibake(m.group()):
                # Find line number
                lineno = text[: m.start()].count("\n") + 1
                moji_hits.append((lineno, m.group()[:40]))

        if moji_hits:
            if args.fix:
                fixed = fix_text(text)
                if fixed != text:
                    with open(path, "w", encoding="utf-8", newline="") as fh:
                        fh.write(fixed)
                    print(f"FIXED  {path}: {len(moji_hits)} mojibake run(s) repaired")
                else:
                    errors.append(f"{path}: mojibake found but auto-fix failed:")
                    for lineno, sample in moji_hits[:3]:
                        errors.append(f"  line {lineno}: {sample!r}")
            else:
                errors.append(f"{path}: mojibake (garbled cyrillic) on {len(moji_hits)} line(s):")
                for lineno, sample in moji_hits[:3]:
                    errors.append(f"  line {lineno}: {sample!r}")

    if errors:
        print("ENCODING ERRORS FOUND:", file=sys.stderr)
        for e in errors:
            print(" ", e, file=sys.stderr)
        return 1

    print(f"OK: encoding check passed (scanned dirs: {', '.join(args.dirs)})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
