#!/usr/bin/env python3
import argparse
import json
import os
import sys
import traceback


def parse_args():
    parser = argparse.ArgumentParser(description="Transcribe extracted audio to JSON.")
    parser.add_argument("--audio-path", help="Absolute path to extracted audio file.")
    parser.add_argument("--output-path", default="", help="Absolute path to the JSON result file.")
    parser.add_argument("--progress-path", default="", help="Optional absolute path to the progress JSON file.")
    parser.add_argument("--backend", default="faster-whisper", help="Transcription backend name.")
    parser.add_argument("--model-name", default="tiny", help="Model name, for example tiny, base, or small.")
    parser.add_argument("--device", default="cpu", help="Inference device: cpu or cuda.")
    parser.add_argument("--compute-type", default="int8", help="Backend compute type.")
    parser.add_argument("--language", default="", help="Optional language hint, for example ru or en.")
    parser.add_argument("--beam-size", type=int, default=5, help="Beam size used by the backend.")
    parser.add_argument(
        "--vad-enabled",
        default="true",
        help="Whether VAD filtering is enabled. Accepts true or false.",
    )
    parser.add_argument(
        "--self-check",
        action="store_true",
        help="Verify that the backend can be imported and print the active local configuration.",
    )
    return parser.parse_args()


def load_backend():
    try:
        from faster_whisper import WhisperModel  # type: ignore
    except ImportError as exc:
        raise RuntimeError(
            "transcription backend is not installed; run scripts/install_transcription_backend.ps1"
        ) from exc

    return WhisperModel


def parse_bool(value):
    normalized = (value or "").strip().lower()
    if normalized in {"1", "true", "yes", "on"}:
        return True
    if normalized in {"0", "false", "no", "off"}:
        return False
    raise RuntimeError(f"invalid boolean value: {value}")


def load_backend_config(args):
    model_name = (args.model_name or os.environ.get("WHISPER_MODEL", "tiny")).strip().lower()
    device = (args.device or os.environ.get("WHISPER_DEVICE", "cpu")).strip().lower()
    compute_type = (args.compute_type or os.environ.get("WHISPER_COMPUTE_TYPE", "int8")).strip().lower()
    backend = (args.backend or "faster-whisper").strip().lower()

    return {
        "backend": backend,
        "model": model_name,
        "device": device,
        "compute_type": compute_type,
        "beam_size": args.beam_size,
        "vad_enabled": parse_bool(args.vad_enabled),
    }


def validate_config(config):
    if config["backend"] != "faster-whisper":
        raise RuntimeError(f"unsupported backend: {config['backend']}")
    if config["model"] not in {"tiny", "base", "small"}:
        raise RuntimeError(f"unsupported model_name: {config['model']}")
    if config["device"] not in {"cpu", "cuda"}:
        raise RuntimeError(f"unsupported device: {config['device']}")
    valid_compute_types = {
        "cpu": {"int8", "float32"},
        "cuda": {"float16", "int8_float16"},
    }
    if config["compute_type"] not in valid_compute_types[config["device"]]:
        raise RuntimeError(
            f"unsupported compute_type for {config['device']}: {config['compute_type']}"
        )
    if config["beam_size"] < 1 or config["beam_size"] > 10:
        raise RuntimeError("beam_size must be between 1 and 10")


def self_check(args):
    load_backend()
    config = load_backend_config(args)
    validate_config(config)

    return {
        "backend": config["backend"],
        "status": "ok",
        "model": config["model"],
        "device": config["device"],
        "compute_type": config["compute_type"],
        "beam_size": config["beam_size"],
        "vad_enabled": config["vad_enabled"],
    }


def write_progress(progress_path, processed_sec, total_sec):
    if not progress_path:
        return

    total_sec = float(total_sec or 0.0)
    processed_sec = float(processed_sec or 0.0)
    percent = 0.0
    if total_sec > 0:
        percent = max(0.0, min(100.0, (processed_sec / total_sec) * 100.0))

    payload = {
        "processed_sec": processed_sec,
        "total_sec": total_sec,
        "percent": percent,
        "is_estimate": total_sec > 0,
    }
    output_dir = os.path.dirname(progress_path) or "."
    os.makedirs(output_dir, exist_ok=True)
    tmp_path = progress_path + ".tmp"
    with open(tmp_path, "w", encoding="utf-8") as progress_file:
        json.dump(payload, progress_file, ensure_ascii=True)
        progress_file.flush()
    os.replace(tmp_path, progress_path)


def transcribe_with_backend(audio_path, language, args):
    whisper_model = load_backend()
    config = load_backend_config(args)
    validate_config(config)
    model = whisper_model(
        config["model"],
        device=config["device"],
        compute_type=config["compute_type"],
    )
    segments, info = model.transcribe(
        audio_path,
        language=language or None,
        beam_size=config["beam_size"],
        vad_filter=config["vad_enabled"],
    )

    total_duration = float(getattr(info, "duration", 0.0) or 0.0)
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
        write_progress(args.progress_path, float(segment.end), total_duration)

    full_text = " ".join(full_text_parts).strip()
    if not full_text:
        raise RuntimeError("transcription backend returned empty text")

    if total_duration > 0:
        write_progress(args.progress_path, total_duration, total_duration)

    return {
        "full_text": full_text,
        "segments": result_segments,
        "language": getattr(info, "language", language or ""),
    }


def main():
    args = parse_args()

    if args.self_check:
        try:
            json.dump(self_check(args), sys.stdout, ensure_ascii=True)
        except Exception as exc:
            print(str(exc), file=sys.stderr)
            return 1
        return 0

    if not args.audio_path:
        print("--audio-path is required unless --self-check is used", file=sys.stderr)
        return 1

    if not args.output_path:
        print("--output-path is required unless --self-check is used", file=sys.stderr)
        return 1

    if not os.path.isfile(args.audio_path):
        print(f"audio file not found: {args.audio_path}", file=sys.stderr)
        return 1

    try:
        result = transcribe_with_backend(args.audio_path, args.language, args)
    except Exception as exc:
        print(f"{type(exc).__name__}: {exc}", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
        return 1

    output_dir = os.path.dirname(args.output_path) or "."
    os.makedirs(output_dir, exist_ok=True)
    tmp_output_path = args.output_path + ".tmp"

    with open(tmp_output_path, "w", encoding="utf-8") as output_file:
        json.dump(
            {
                "full_text": result["full_text"],
                "segments": result["segments"],
            },
            output_file,
            ensure_ascii=True,
        )
        output_file.flush()

    os.replace(tmp_output_path, args.output_path)
    print("ok", file=sys.stdout)
    return 0


if __name__ == "__main__":
    sys.exit(main())
