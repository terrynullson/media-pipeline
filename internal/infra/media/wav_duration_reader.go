package media

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type WAVDurationReader struct{}

func NewWAVDurationReader() *WAVDurationReader {
	return &WAVDurationReader{}
}

func (r *WAVDurationReader) ReadDuration(audioPath string) (time.Duration, error) {
	file, err := os.Open(audioPath)
	if err != nil {
		return 0, fmt.Errorf("open wav file: %w", err)
	}
	defer file.Close()

	header := make([]byte, 12)
	if _, err := io.ReadFull(file, header); err != nil {
		return 0, fmt.Errorf("read wav header: %w", err)
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return 0, fmt.Errorf("unsupported wav container")
	}

	var byteRate uint32
	var dataSize uint32

	for {
		chunkHeader := make([]byte, 8)
		if _, err := io.ReadFull(file, chunkHeader); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return 0, fmt.Errorf("read wav chunk header: %w", err)
		}

		chunkID := string(chunkHeader[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])

		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return 0, fmt.Errorf("invalid fmt chunk size %d", chunkSize)
			}
			payload := make([]byte, chunkSize)
			if _, err := io.ReadFull(file, payload); err != nil {
				return 0, fmt.Errorf("read fmt chunk: %w", err)
			}
			byteRate = binary.LittleEndian.Uint32(payload[8:12])
		case "data":
			dataSize = chunkSize
			if _, err := file.Seek(int64(chunkSize), io.SeekCurrent); err != nil {
				return 0, fmt.Errorf("skip data chunk: %w", err)
			}
		default:
			if _, err := file.Seek(int64(chunkSize), io.SeekCurrent); err != nil {
				return 0, fmt.Errorf("skip %s chunk: %w", strings.TrimSpace(chunkID), err)
			}
		}

		if chunkSize%2 == 1 {
			if _, err := file.Seek(1, io.SeekCurrent); err != nil {
				return 0, fmt.Errorf("skip wav padding byte: %w", err)
			}
		}
	}

	if byteRate == 0 {
		return 0, fmt.Errorf("wav byte rate is missing")
	}
	if dataSize == 0 {
		return 0, fmt.Errorf("wav data chunk is missing")
	}

	return time.Duration(int64(dataSize) * int64(time.Second) / int64(byteRate)), nil
}
