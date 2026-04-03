package ports

import "time"

type AudioDurationReader interface {
	ReadDuration(audioPath string) (time.Duration, error)
}
