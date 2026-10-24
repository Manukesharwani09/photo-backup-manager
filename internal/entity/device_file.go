package entity

import "github.com/Manukesharwani09/photo-backup-manager/internal/enum"

// DeviceFile is a file at a path on a phone, pointing at its content (Media).
type DeviceFile struct {
	BaseModel
	DeviceID        int64           `db:"device_id" json:"deviceId"`
	MediaID         int64           `db:"media_id" json:"mediaId"`
	RelativePath    string          `db:"relative_path" json:"relativePath"`
	DisplayName     string          `db:"display_name" json:"displayName"`
	Size            int64           `db:"size" json:"size"`
	Source          enum.FileSource `db:"source" json:"source"`
	State           enum.FileState  `db:"state" json:"state"`
	PresentOnDevice bool            `db:"present_on_device" json:"presentOnDevice"`
}
