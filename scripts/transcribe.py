#!/usr/bin/env python3
import argparse
import json
import os
import sys
import traceback


def parse_args():
    parser = argparse.ArgumentParser(description="Transcribe extracted audio to JSON.")
    parser.add_argument("--audio-path", help="Absolute path to extracted audio file.")
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
    model_name = args.model_name or os.environ.get("WHISPER_MODEL", "tiny")
    device = args.device or os.environ.get("WHISPER_DEVICE", "cpu")
    compute_type = args.compute_type or os.environ.get("WHISPER_COMPUTE_TYPE", "int8")

    return {
        "backend": args.backend,
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

    if not os.path.isfile(args.audio_path):
        print(f"audio file not found: {args.audio_path}", file=sys.stderr)
        return 1

    try:
        result = transcribe_with_backend(args.audio_path, args.language, args)
    except Exception as exc:
        print(f"{type(exc).__name__}: {exc}", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
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
