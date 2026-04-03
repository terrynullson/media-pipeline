package transcription

import "testing"

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
