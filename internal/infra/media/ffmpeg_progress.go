package media

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"media-pipeline/internal/domain/ports"
)

var ffmpegDurationPattern = regexp.MustCompile(`Duration:\s*(\d{2}):(\d{2}):(\d{2}(?:\.\d+)?)`)

func runFFmpegWithProgress(
	ctx context.Context,
	binary string,
	args []string,
	onProgress func(ports.TranscriptionProgress),
) (string, error) {
	cmd := exec.CommandContext(ctx, binary, args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("create ffmpeg stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("create ffmpeg stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start ffmpeg: %w", err)
	}

	var stderr bytes.Buffer
	var totalSec float64
	var totalMu sync.RWMutex
	var wg sync.WaitGroup
	var stdoutErr error
	var stderrErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		stdoutErr = consumeFFmpegProgress(stdoutPipe, func(processedSec float64) {
			totalMu.RLock()
			currentTotal := totalSec
			totalMu.RUnlock()
			if onProgress == nil || currentTotal <= 0 || processedSec < 0 {
				return
			}
			percent := math.Min(100, math.Max(0, (processedSec/currentTotal)*100))
			onProgress(ports.TranscriptionProgress{
				ProcessedSec: processedSec,
				TotalSec:     currentTotal,
				Percent:      percent,
			})
		})
	}()
	go func() {
		defer wg.Done()
		stderrErr = consumeFFmpegStderr(stderrPipe, &stderr, func(parsedTotalSec float64) {
			if parsedTotalSec <= 0 {
				return
			}
			totalMu.Lock()
			if totalSec <= 0 {
				totalSec = parsedTotalSec
			}
			totalMu.Unlock()
		})
	}()

	waitErr := cmd.Wait()
	wg.Wait()

	if stdoutErr != nil {
		return stderr.String(), stdoutErr
	}
	if stderrErr != nil {
		return stderr.String(), stderrErr
	}
	if waitErr != nil {
		return stderr.String(), waitErr
	}

	totalMu.RLock()
	finalTotal := totalSec
	totalMu.RUnlock()
	if onProgress != nil && finalTotal > 0 {
		onProgress(ports.TranscriptionProgress{
			ProcessedSec: finalTotal,
			TotalSec:     finalTotal,
			Percent:      100,
		})
	}

	return stderr.String(), nil
}

func consumeFFmpegProgress(reader io.Reader, onProcessedSec func(float64)) error {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "out_time=") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "out_time="))
		if value == "" || value == "N/A" {
			continue
		}
		processedSec, err := parseFFmpegTimestamp(value)
		if err != nil {
			continue
		}
		onProcessedSec(processedSec)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read ffmpeg progress: %w", err)
	}

	return nil
}

func consumeFFmpegStderr(reader io.Reader, stderr *bytes.Buffer, onDuration func(float64)) error {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		stderr.WriteString(line)
		stderr.WriteByte('\n')
		if matches := ffmpegDurationPattern.FindStringSubmatch(line); len(matches) == 4 {
			durationSec, err := ffmpegDurationMatchesToSeconds(matches[1], matches[2], matches[3])
			if err == nil {
				onDuration(durationSec)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read ffmpeg stderr: %w", err)
	}

	return nil
}

func parseFFmpegTimestamp(value string) (float64, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid ffmpeg timestamp %q", value)
	}

	return ffmpegDurationMatchesToSeconds(parts[0], parts[1], parts[2])
}

func ffmpegDurationMatchesToSeconds(hoursRaw string, minutesRaw string, secondsRaw string) (float64, error) {
	hours, err := strconv.Atoi(hoursRaw)
	if err != nil {
		return 0, err
	}
	minutes, err := strconv.Atoi(minutesRaw)
	if err != nil {
		return 0, err
	}
	seconds, err := strconv.ParseFloat(secondsRaw, 64)
	if err != nil {
		return 0, err
	}

	return float64(hours*3600+minutes*60) + seconds, nil
}
