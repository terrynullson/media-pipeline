#!/usr/bin/env python3
import argparse
import json
import os
import sys


def parse_args():
    parser = argparse.ArgumentParser(description="Transcribe extracted audio to JSON.")
    parser.add_argument("--audio-path", required=True, help="Absolute path to extracted audio file.")
    parser.add_argument("--language", default="", help="Optional language hint, for example ru or en.")
    return parser.parse_args()


def transcribe_with_backend(audio_path, language):
    try:
        from faster_whisper import WhisperModel  # type: ignore
    except ImportError as exc:
        raise RuntimeError(
            "transcription backend is not installed; install faster-whisper or replace scripts/transcribe.py"
        ) from exc

    model_name = os.environ.get("WHISPER_MODEL", "tiny")
    model = WhisperModel(model_name, device="cpu", compute_type="int8")
    segments, info = model.transcribe(audio_path, language=language or None)

    result_segments = []
    full_text_parts = []
    for segment in segments:
        text = (segment.text or "").strip()
        if not text:
            continue

        item = {
            "start_sec": float(segment.start),
            "end_sec": float(segment.end),
            "text": text,
        }
        if getattr(segment, "avg_logprob", None) is not None:
            item["confidence"] = float(segment.avg_logprob)

        full_text_parts.append(text)
        result_segments.append(item)

    full_text = " ".join(full_text_parts).strip()
    if not full_text:
        raise RuntimeError("transcription backend returned empty text")

    return {
        "full_text": full_text,
        "segments": result_segments,
        "language": getattr(info, "language", language or ""),
    }


def main():
    args = parse_args()

    if not os.path.isfile(args.audio_path):
        print(f"audio file not found: {args.audio_path}", file=sys.stderr)
        return 1

    try:
        result = transcribe_with_backend(args.audio_path, args.language)
    except Exception as exc:
        print(str(exc), file=sys.stderr)
        return 1

    json.dump(
        {
            "full_text": result["full_text"],
            "segments": result["segments"],
        },
        sys.stdout,
        ensure_ascii=True,
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
