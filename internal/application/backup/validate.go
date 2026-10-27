package backup

import (
	"encoding/hex"
	"strconv"
	"time"

	"github.com/Manukesharwani09/photo-backup-manager/internal/enum"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
	"github.com/Manukesharwani09/photo-backup-manager/internal/planner"
)

// toFileMeta validates one reported file. A non-empty problem rejects only that file.
func toFileMeta(f SyncFile) (m repository.FileMeta, problem string) {
	m = repository.FileMeta{RelativePath: planner.NormalizeFolder(f.RelativePath), DisplayName: f.DisplayName, Size: f.Size,
		Width: f.Width, Height: f.Height, GPSLat: f.GPSLat, GPSLng: f.GPSLng, Source: f.Source}
	where := m.RelativePath + "/" + m.DisplayName + ": "
	if m.DisplayName == "" {
		return m, "file with empty display_name"
	}
	sum, err := hex.DecodeString(f.SHA256)
	if err != nil || len(sum) != 32 {
		return m, where + "sha256 must be 64 hex characters"
	}
	m.SHA256 = sum
	if m.Size < 0 {
		return m, where + "negative size"
	}
	if m.Source == "" {
		m.Source = string(enum.FILE_SOURCE_OTHER)
	}
	if !enum.FileSource(m.Source).IsValid() {
		return m, where + "unknown source " + m.Source
	}
	if f.DateTaken != nil && *f.DateTaken != "" {
		t, err := time.Parse(time.RFC3339, *f.DateTaken)
		if err != nil {
			return m, where + "date_taken must be RFC 3339"
		}
		m.DateTaken = &t
	}
	if f.PHash != nil && *f.PHash != "" {
		v, err := strconv.ParseUint(*f.PHash, 16, 64)
		if err != nil || len(*f.PHash) != 16 {
			return m, where + "phash must be 16 hex characters"
		}
		signed := int64(v)
		m.PHash = &signed
	}
	if (m.GPSLat == nil) != (m.GPSLng == nil) ||
		(m.GPSLat != nil && (*m.GPSLat < -90 || *m.GPSLat > 90 || *m.GPSLng < -180 || *m.GPSLng > 180)) {
		return m, where + "invalid GPS"
	}
	return m, ""
}
