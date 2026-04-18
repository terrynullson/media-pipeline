package autoupload

import "time"

type ImportKey struct {
	RelativePath  string
	SizeBytes     int64
	ModifiedAtUTC time.Time
}

type ImportStatus string

const (
	ImportStatusImporting ImportStatus = "importing"
	ImportStatusImported  ImportStatus = "imported"
)

type BeginImportResult struct {
	Started bool
	Status  ImportStatus
	MediaID *int64
}
