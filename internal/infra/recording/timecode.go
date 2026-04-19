// Package recording derives broadcast metadata (source/recorder, absolute
// timecode) from a raw recording filename.
//
// The expected filename layout is:
//
//	<source>_<YY>.<MM>.<DD>_<HH>.<MM>.<SS>[.<ff>].<ext>
//
// e.g. Recorder_1_25.02.05_00.00.00.00.mp4 → source="Recorder_1",
// started=2025-02-05 00:00:00.00 UTC. The trailing .ff group is treated as
// hundredths of a second (frames at 100fps would behave the same to the
// nearest 10ms — for analytic windows that resolution is more than enough).
//
// Returning a *Result of nil means "filename does not match the recorder
// convention"; that is not an error — callers should keep going and leave the
// airtime fields blank.
package recording

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Result holds the structured fields parsed from a recorder filename.
type Result struct {
	SourceName        string
	StartedAtUTC      time.Time
	RawRecordingLabel string // YY.MM.DD_HH.MM.SS[.ff] — preserved for debugging
}

// Capture groups: 1=source, 2=YY, 3=MM, 4=DD, 5=HH, 6=MM, 7=SS, 8=ff (optional)
var filenameRe = regexp.MustCompile(
	`^(?P<source>.+?)_(?P<yy>\d{2})\.(?P<mo>\d{2})\.(?P<dd>\d{2})_(?P<hh>\d{2})\.(?P<mm>\d{2})\.(?P<ss>\d{2})(?:\.(?P<ff>\d{2}))?$`,
)

// ParseFilename inspects a raw filename (with or without an extension) and
// returns the parsed result, or (nil, nil) if it does not look like a
// recorder filename. A non-nil error is reserved for "looks like the format
// but the digits are nonsense" — callers should log it and continue.
func ParseFilename(name string) (*Result, error) {
	stem := strings.TrimSpace(name)
	if stem == "" {
		return nil, nil
	}
	stem = strings.TrimSuffix(stem, filepath.Ext(stem))

	match := filenameRe.FindStringSubmatch(stem)
	if match == nil {
		return nil, nil
	}

	source := strings.TrimSpace(match[filenameRe.SubexpIndex("source")])
	if source == "" {
		return nil, nil
	}

	yy, _ := strconv.Atoi(match[filenameRe.SubexpIndex("yy")])
	mo, _ := strconv.Atoi(match[filenameRe.SubexpIndex("mo")])
	dd, _ := strconv.Atoi(match[filenameRe.SubexpIndex("dd")])
	hh, _ := strconv.Atoi(match[filenameRe.SubexpIndex("hh")])
	mm, _ := strconv.Atoi(match[filenameRe.SubexpIndex("mm")])
	ss, _ := strconv.Atoi(match[filenameRe.SubexpIndex("ss")])

	ffRaw := match[filenameRe.SubexpIndex("ff")]
	ff := 0
	if ffRaw != "" {
		ff, _ = strconv.Atoi(ffRaw)
	}

	// Two-digit year: 00..69 → 2000–2069, 70..99 → 1970–1999. Same rule as the
	// Win32 SHFormatDateTime convention; safe for broadcast filenames seen in
	// practice and avoids surprising Y2K-style flips.
	year := 2000 + yy
	if yy >= 70 {
		year = 1900 + yy
	}

	if mo < 1 || mo > 12 || dd < 1 || dd > 31 || hh > 23 || mm > 59 || ss > 59 || ff > 99 {
		return nil, fmt.Errorf("recording: out-of-range timecode in %q", stem)
	}

	// .ff is hundredths of a second.
	nsec := ff * 10 * int(time.Millisecond)

	t := time.Date(year, time.Month(mo), dd, hh, mm, ss, nsec, time.UTC)

	return &Result{
		SourceName:        source,
		StartedAtUTC:      t,
		RawRecordingLabel: stem[len(source)+1:],
	}, nil
}
