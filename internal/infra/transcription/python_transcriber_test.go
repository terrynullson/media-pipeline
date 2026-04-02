package transcription

import "testing"

func TestParseTranscriptionOutput(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"full_text": "privet mir",
		"segments": [
			{"start_sec": 0, "end_sec": 1.5, "text": "privet", "confidence": 0.95},
			{"start_sec": 1.5, "end_sec": 3.0, "text": "mir"}
		]
	}`)

	result, err := ParseTranscriptionOutput(payload)
	if err != nil {
		t.Fatalf("ParseTranscriptionOutput() error = %v", err)
	}
	if result.FullText != "privet mir" {
		t.Fatalf("full text = %q, want %q", result.FullText, "privet mir")
	}
	if len(result.Segments) != 2 {
		t.Fatalf("segments count = %d, want 2", len(result.Segments))
	}
	if result.Segments[0].Confidence == nil {
		t.Fatal("segments[0].confidence = nil, want value")
	}
}

func TestParseTranscriptionOutputRejectsInvalidSegment(t *testing.T) {
	t.Parallel()

	payload := []byte(`{
		"full_text": "privet mir",
		"segments": [
			{"start_sec": 5, "end_sec": 3, "text": "broken"}
		]
	}`)

	if _, err := ParseTranscriptionOutput(payload); err == nil {
		t.Fatal("ParseTranscriptionOutput() error = nil, want validation error")
	}
}
