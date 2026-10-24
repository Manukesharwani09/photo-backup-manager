package entity

import (
	"time"

	"github.com/Manukesharwani09/photo-backup-manager/internal/enum"
)

// BackupRecord is a Media stored in a Google account. At most one live record (uploaded/verified)
// exists per Media: the database-level "never upload twice" guarantee.
type BackupRecord struct {
	BaseModel
	MediaID            int64             `db:"media_id" json:"mediaId"`
	AccountID          int64             `db:"account_id" json:"accountId"`
	GPhotosMediaItemID *string           `db:"gphotos_media_item_id" json:"mediaItemId"`
	ProductURL         *string           `db:"product_url" json:"productUrl"`
	AlbumID            *int64            `db:"album_id" json:"albumId"`
	Status             enum.BackupStatus `db:"status" json:"status"`
	UploadedAt         time.Time         `db:"uploaded_at" json:"uploadedAt"`
	VerifiedAt         *time.Time        `db:"verified_at" json:"verifiedAt"`
	LastCheckedAt      *time.Time        `db:"last_checked_at" json:"lastCheckedAt"`
	MarkedForDeleteAt  *time.Time        `db:"marked_for_delete_at" json:"markedForDeleteAt"`
}
