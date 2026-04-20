package recording

import (
	"testing"
	"time"
)

func TestParseFilename_RecorderConvention(t *testing.T) {
	t.Parallel()

	res, err := ParseFilename("Recorder_1_25.02.05_00.00.00.00.mp4")
	if err != nil {
		t.Fatalf("ParseFilename error = %v", err)
	}
	if res == nil {
		t.Fatal("ParseFilename = nil, want parsed result")
	}
	if res.SourceName != "Recorder_1" {
		t.Errorf("SourceName = %q, want Recorder_1", res.SourceName)
	}
	want := time.Date(2025, 2, 5, 0, 0, 0, 0, time.UTC)
	if !res.StartedAtUTC.Equal(want) {
		t.Errorf("StartedAtUTC = %s, want %s", res.StartedAtUTC, want)
	}
	if res.RawRecordingLabel != "25.02.05_00.00.00.00" {
		t.Errorf("RawRecordingLabel = %q", res.RawRecordingLabel)
	}
}

func TestParseFilename_Hundredths(t *testing.T) {
	t.Parallel()

	res, err := ParseFilename("Recorder_2_25.04.19_14.30.05.50.wav")
	if err != nil || res == nil {
		t.Fatalf("ParseFilename error = %v, res = %v", err, res)
	}
	want := time.Date(2025, 4, 19, 14, 30, 5, 500*int(time.Millisecond), time.UTC)
	if !res.StartedAtUTC.Equal(want) {
		t.Errorf("StartedAtUTC = %s, want %s", res.StartedAtUTC, want)
	}
}

func TestParseFilename_NoFractional(t *testing.T) {
	t.Parallel()

	res, err := ParseFilename("Studio-A_24.12.31_23.59.59.mp4")
	if err != nil || res == nil {
		t.Fatalf("ParseFilename error = %v, res = %v", err, res)
	}
	if res.SourceName != "Studio-A" {
		t.Errorf("SourceName = %q", res.SourceName)
	}
}

func TestParseFilename_NotRecording(t *testing.T) {
	t.Parallel()

	cases := []string{
		"random.mp4",
		"meeting_notes.txt",
		"",
		"Recorder_1_xx.yy.zz_aa.bb.cc.mp4",
	}
	for _, name := range cases {
		res, err := ParseFilename(name)
		if err != nil {
			t.Errorf("ParseFilename(%q) error = %v", name, err)
		}
		if res != nil {
			t.Errorf("ParseFilename(%q) = %+v, want nil", name, res)
		}
	}
}

func TestParseFilename_OutOfRange(t *testing.T) {
	t.Parallel()

	if _, err := ParseFilename("Recorder_1_25.13.05_00.00.00.mp4"); err == nil {
		t.Error("expected error for month=13")
	}
}
