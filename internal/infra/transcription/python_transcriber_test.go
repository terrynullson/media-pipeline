package transcription

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"media-pipeline/internal/domain/ports"
	domaintranscription "media-pipeline/internal/domain/transcription"
)

func TestParseTranscriptionOutput(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"full_text":"hello world","segments":[{"start_sec":0,"end_sec":1.2,"text":"hello","confidence":0.91},{"start_sec":1.2,"end_sec":2.0,"text":"world"}]}`)

	parsed, err := ParseTranscriptionOutput(payload)
	if err != nil {
		t.Fatalf("ParseTranscriptionOutput() error = %v", err)
	}
	if parsed.FullText != "hello world" {
		t.Fatalf("full text = %q, want hello world", parsed.FullText)
	}
	if len(parsed.Segments) != 2 {
		t.Fatalf("segments = %d, want 2", len(parsed.Segments))
	}
	if parsed.Segments[0].Confidence == nil {
		t.Fatal("segments[0].confidence = nil, want value")
	}
	if parsed.Segments[1].Confidence != nil {
		t.Fatal("segments[1].confidence != nil, want omitted confidence")
	}
}

func TestParseTranscriptionOutputRejectsEmptyFullText(t *testing.T) {
	t.Parallel()

	_, err := ParseTranscriptionOutput([]byte(`{"full_text":"","segments":[{"start_sec":0,"end_sec":1,"text":"x"}]}`))
	if err == nil {
		t.Fatal("ParseTranscriptionOutput() error = nil, want validation error")
	}
}

func TestReadTranscriptionResult(t *testing.T) {
	t.Parallel()

	resultPath := filepath.Join(t.TempDir(), "result.json")
	if err := os.WriteFile(resultPath, []byte(`{"full_text":"privet mir","segments":[{"start_sec":0,"end_sec":1.5,"text":"privet"},{"start_sec":1.5,"end_sec":3,"text":"mir"}]}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	parsed, err := readTranscriptionResult(resultPath)
	if err != nil {
		t.Fatalf("readTranscriptionResult() error = %v", err)
	}
	if parsed.FullText != "privet mir" {
		t.Fatalf("full text = %q, want privet mir", parsed.FullText)
	}
	if len(parsed.Segments) != 2 {
		t.Fatalf("segments = %d, want 2", len(parsed.Segments))
	}
}

func TestReadTranscriptionResultMissingFile(t *testing.T) {
	t.Parallel()

	_, err := readTranscriptionResult(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("readTranscriptionResult() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "result file not found") {
		t.Fatalf("error = %v, want missing result file", err)
	}
}

func TestPythonTranscriberUsesResultFileTransport(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("batch-script subprocess test is only enabled on Windows")
	}

	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "fake-python.cmd")
	scriptPath := filepath.Join(tempDir, "placeholder.py")
	if err := os.WriteFile(scriptPath, []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(scriptPath) error = %v", err)
	}

	batch := "@echo off\r\n" +
		"setlocal EnableDelayedExpansion\r\n" +
		"set OUTPUT=\r\n" +
		":loop\r\n" +
		"if \"%~1\"==\"\" goto done\r\n" +
		"if /I \"%~1\"==\"--output-path\" (\r\n" +
		"  set OUTPUT=%~2\r\n" +
		"  shift\r\n" +
		")\r\n" +
		"shift\r\n" +
		"goto loop\r\n" +
		":done\r\n" +
		"if \"%OUTPUT%\"==\"\" exit /b 9\r\n" +
		"echo diagnostic line\r\n" +
		"> \"%OUTPUT%\" echo {\"full_text\":\"hello world\",\"segments\":[{\"start_sec\":0,\"end_sec\":1.1,\"text\":\"hello\"},{\"start_sec\":1.1,\"end_sec\":2.2,\"text\":\"world\",\"confidence\":0.8}]}\r\n"
	if err := os.WriteFile(binaryPath, []byte(batch), 0o644); err != nil {
		t.Fatalf("WriteFile(binaryPath) error = %v", err)
	}

	transcriber := NewPythonTranscriber(binaryPath, scriptPath, slog.New(slog.NewTextHandler(io.Discard, nil)))

	output, err := transcriber.Transcribe(context.Background(), ports.TranscribeInput{
		AudioPath: filepath.Join(tempDir, "audio.wav"),
		Settings: domaintranscription.Settings{
			Backend:     domaintranscription.BackendFasterWhisper,
			ModelName:   "tiny",
			Device:      "cpu",
			ComputeType: "int8",
			Language:    "ru",
			BeamSize:    5,
			VADEnabled:  true,
		},
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if output.FullText != "hello world" {
		t.Fatalf("full text = %q, want hello world", output.FullText)
	}
	if len(output.Segments) != 2 {
		t.Fatalf("segments = %d, want 2", len(output.Segments))
	}
	if output.Segments[1].Confidence == nil {
		t.Fatal("segments[1].confidence = nil, want value")
	}
}

func TestPythonTranscriberRetriesFloat16FallbackOnUnsupportedBackend(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("batch-script subprocess test is only enabled on Windows")
	}

	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "fake-python.cmd")
	scriptPath := filepath.Join(tempDir, "placeholder.py")
	audioPath := filepath.Join(tempDir, "audio.wav")
	if err := os.WriteFile(scriptPath, []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(scriptPath) error = %v", err)
	}
	if err := os.WriteFile(audioPath, []byte("wav"), 0o644); err != nil {
		t.Fatalf("WriteFile(audioPath) error = %v", err)
	}

	batch := "@echo off\r\n" +
		"setlocal EnableDelayedExpansion\r\n" +
		"set OUTPUT=\r\n" +
		"set COMPUTE=\r\n" +
		":loop\r\n" +
		"if \"%~1\"==\"\" goto done\r\n" +
		"if /I \"%~1\"==\"--output-path\" (\r\n" +
		"  set OUTPUT=%~2\r\n" +
		"  shift\r\n" +
		")\r\n" +
		"if /I \"%~1\"==\"--compute-type\" (\r\n" +
		"  set COMPUTE=%~2\r\n" +
		"  shift\r\n" +
		")\r\n" +
		"shift\r\n" +
		"goto loop\r\n" +
		":done\r\n" +
		"if \"%COMPUTE%\"==\"float16\" (\r\n" +
		"  1>&2 echo ValueError: Requested float16 compute type, but the target device or backend do not support efficient float16 computation.\r\n" +
		"  exit /b 1\r\n" +
		")\r\n" +
		"if \"%OUTPUT%\"==\"\" exit /b 9\r\n" +
		"> \"%OUTPUT%\" echo {\"full_text\":\"fallback ok\",\"segments\":[{\"start_sec\":0,\"end_sec\":1.0,\"text\":\"fallback ok\"}]}\r\n"
	if err := os.WriteFile(binaryPath, []byte(batch), 0o644); err != nil {
		t.Fatalf("WriteFile(binaryPath) error = %v", err)
	}

	transcriber := NewPythonTranscriber(binaryPath, scriptPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	output, err := transcriber.Transcribe(context.Background(), ports.TranscribeInput{
		AudioPath: audioPath,
		Settings: domaintranscription.Settings{
			Backend:     domaintranscription.BackendFasterWhisper,
			ModelName:   "base",
			Device:      "cuda",
			ComputeType: "float16",
			Language:    "ru",
			BeamSize:    5,
			VADEnabled:  true,
		},
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if output.FullText != "fallback ok" {
		t.Fatalf("full text = %q, want fallback ok", output.FullText)
	}
	if len(output.Segments) != 1 {
		t.Fatalf("segments = %d, want 1", len(output.Segments))
	}
}

func TestPythonTranscriberRetriesInt8Float16FallbackOnUnsupportedBackend(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("batch-script subprocess test is only enabled on Windows")
	}

	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "fake-python.cmd")
	scriptPath := filepath.Join(tempDir, "placeholder.py")
	audioPath := filepath.Join(tempDir, "audio.wav")
	if err := os.WriteFile(scriptPath, []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile(scriptPath) error = %v", err)
	}
	if err := os.WriteFile(audioPath, []byte("wav"), 0o644); err != nil {
		t.Fatalf("WriteFile(audioPath) error = %v", err)
	}

	batch := "@echo off\r\n" +
		"setlocal EnableDelayedExpansion\r\n" +
		"set OUTPUT=\r\n" +
		"set COMPUTE=\r\n" +
		":loop\r\n" +
		"if \"%~1\"==\"\" goto done\r\n" +
		"if /I \"%~1\"==\"--output-path\" (\r\n" +
		"  set OUTPUT=%~2\r\n" +
		"  shift\r\n" +
		")\r\n" +
		"if /I \"%~1\"==\"--compute-type\" (\r\n" +
		"  set COMPUTE=%~2\r\n" +
		"  shift\r\n" +
		")\r\n" +
		"shift\r\n" +
		"goto loop\r\n" +
		":done\r\n" +
		"if \"%COMPUTE%\"==\"int8_float16\" (\r\n" +
		"  1>&2 echo ValueError: Requested int8_float16 compute type, but the target device or backend do not support efficient int8_float16 computation.\r\n" +
		"  exit /b 1\r\n" +
		")\r\n" +
		"if \"%OUTPUT%\"==\"\" exit /b 9\r\n" +
		"> \"%OUTPUT%\" echo {\"full_text\":\"fallback float32 ok\",\"segments\":[{\"start_sec\":0,\"end_sec\":1.0,\"text\":\"fallback float32 ok\"}]}\r\n"
	if err := os.WriteFile(binaryPath, []byte(batch), 0o644); err != nil {
		t.Fatalf("WriteFile(binaryPath) error = %v", err)
	}

	transcriber := NewPythonTranscriber(binaryPath, scriptPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	output, err := transcriber.Transcribe(context.Background(), ports.TranscribeInput{
		AudioPath: audioPath,
		Settings: domaintranscription.Settings{
			Backend:     domaintranscription.BackendFasterWhisper,
			ModelName:   "base",
			Device:      "cuda",
			ComputeType: "int8_float16",
			Language:    "ru",
			BeamSize:    5,
			VADEnabled:  true,
		},
	})
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if output.FullText != "fallback float32 ok" {
		t.Fatalf("full text = %q, want fallback float32 ok", output.FullText)
	}
	if len(output.Segments) != 1 {
		t.Fatalf("segments = %d, want 1", len(output.Segments))
	}
}
