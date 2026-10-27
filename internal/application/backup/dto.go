package backup

import "github.com/Manukesharwani09/photo-backup-manager/internal/application/account"

type CreateDeviceRequest struct {
	Model string `json:"model"`
}

type CreateDeviceResponse struct {
	DeviceID int64 `json:"device_id"`
}

// SyncFile is what the phone reports for one media file.
type SyncFile struct {
	RelativePath string   `json:"relative_path"`
	DisplayName  string   `json:"display_name"`
	Size         int64    `json:"size"`
	SHA256       string   `json:"sha256"` // hex
	DateTaken    *string  `json:"date_taken"`
	Width        *int     `json:"width"`
	Height       *int     `json:"height"`
	GPSLat       *float64 `json:"gps_lat"`
	GPSLng       *float64 `json:"gps_lng"`
	PHash        *string  `json:"phash"` // optional 64-bit perceptual hash, 16 hex characters
	Source       string   `json:"source"`
}

type SyncRequest struct {
	DeviceID int64      `json:"device_id"`
	Files    []SyncFile `json:"files"`
}

// PlanItem tells the phone to upload one file's bytes to an account.
type PlanItem struct {
	FileID       int64  `json:"file_id"`
	SHA256       string `json:"sha256"`
	RelativePath string `json:"relative_path"`
	DisplayName  string `json:"display_name"`
	AccountID    int64  `json:"account_id"`
	AlbumTitle   string `json:"album_title"`
}

// WaitingItem is a pending file that can't be uploaded yet, with why.
type WaitingItem struct {
	FileID int64  `json:"file_id"`
	Reason string `json:"reason"`
}

type SyncResponse struct {
	FirstScan bool                          `json:"first_scan"`
	Counts    map[string]int                `json:"counts"`
	Plan      []PlanItem                    `json:"plan"`
	Waiting   []WaitingItem                 `json:"waiting"`
	Rejected  []string                      `json:"rejected"` // input errors, one line per bad file
	Tokens    map[int64]account.AccessToken `json:"tokens"`   // short-lived upload tokens for the accounts in Plan
}

type UploadItem struct {
	FileID      int64  `json:"file_id"`
	AccountID   int64  `json:"account_id"`
	UploadToken string `json:"upload_token"`
}

type UploadsRequest struct {
	DeviceID int64        `json:"device_id"`
	Items    []UploadItem `json:"items"`
}

// Upload result statuses.
const (
	UPLOAD_BACKED_UP         = "backed_up"
	UPLOAD_ALREADY_BACKED_UP = "already_backed_up"
	UPLOAD_ACCOUNT_FULL      = "account_full"
	UPLOAD_FAILED            = "failed"
	UPLOAD_INVALID           = "invalid"
)

type UploadResult struct {
	FileID      int64  `json:"file_id"`
	Status      string `json:"status"`
	MediaItemID string `json:"media_item_id,omitempty"`
	ProductURL  string `json:"product_url,omitempty"`
	Verified    bool   `json:"verified,omitempty"`
	Message     string `json:"message,omitempty"`
}

type UploadsResponse struct {
	Results []*UploadResult `json:"results"`
}
