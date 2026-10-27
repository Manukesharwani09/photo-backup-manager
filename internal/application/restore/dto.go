package restore

import (
	"time"

	"github.com/Manukesharwani09/photo-backup-manager/internal/application/account"
)

type PlanRequest struct {
	DeviceID int64  `json:"device_id"`
	Folder   string `json:"folder"` // includes sub-folders
}

// Item is one photo to download back into its original phone folder.
type Item struct {
	MediaID      int64      `json:"media_id"`
	RelativePath string     `json:"relative_path"`
	DisplayName  string     `json:"display_name"`
	Size         int64      `json:"size"`
	AccountID    int64      `json:"account_id"`
	DownloadURL  string     `json:"download_url"` // valid ~60 minutes; fetch with the account's access token
	MimeType     string     `json:"mime_type"`
	DateTaken    *time.Time `json:"date_taken,omitempty"`
	GPSLat       *float64   `json:"gps_lat,omitempty"` // the phone writes these back: Google strips GPS on download
	GPSLng       *float64   `json:"gps_lng,omitempty"`
}

type PlanResponse struct {
	Items           []Item                        `json:"items"`
	AlreadyOnDevice int                           `json:"already_on_device"`
	MissingInGoogle []string                      `json:"missing_in_google"` // deleted in Google Photos; can't restore
	Tokens          map[int64]account.AccessToken `json:"tokens"`
}

// RestoredFile is a file the phone wrote back, with the hash of the bytes it actually received.
type RestoredFile struct {
	MediaID      int64  `json:"media_id"`
	RelativePath string `json:"relative_path"`
	DisplayName  string `json:"display_name"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
}

type DoneRequest struct {
	DeviceID int64          `json:"device_id"`
	Items    []RestoredFile `json:"items"`
}

type DoneResponse struct {
	Linked   int      `json:"linked"`
	Rejected []string `json:"rejected"`
}
