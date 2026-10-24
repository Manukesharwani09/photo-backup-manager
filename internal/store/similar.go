package store

import (
	"context"
	"time"

	"github.com/Manukesharwani09/photo-backup-manager/internal/plan"
)

// SimilarInputs returns every media of the user that is backed up or still on a phone,
// with what plan.SimilarGroups compares. The name is that of one of its phone files.
func (s *Store) SimilarInputs(ctx context.Context, userID int64) ([]plan.Photo, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT m.id, m.phash, m.date_taken, m.width, m.height,
		       (SELECT f.display_name FROM device_file f WHERE f.media_id = m.id ORDER BY f.id LIMIT 1)
		FROM media m
		WHERE m.user_id = $1 AND (
		  EXISTS (SELECT 1 FROM backup_record b WHERE b.media_id = m.id AND b.status IN ('uploaded', 'verified')) OR
		  EXISTS (SELECT 1 FROM device_file f WHERE f.media_id = m.id AND f.present_on_device))
		ORDER BY m.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []plan.Photo
	for rows.Next() {
		var p plan.Photo
		var ph *int64
		var name *string
		if err := rows.Scan(&p.MediaID, &ph, &p.DateTaken, &p.Width, &p.Height, &name); err != nil {
			return nil, err
		}
		if ph != nil {
			u := uint64(*ph)
			p.PHash = &u
		}
		if name != nil {
			p.Name = *name
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MediaDetail describes one media for the Similar photos screen.
type MediaDetail struct {
	MediaID         int64      `json:"media_id"`
	DateTaken       *time.Time `json:"date_taken,omitempty"`
	Width           *int       `json:"width,omitempty"`
	Height          *int       `json:"height,omitempty"`
	Paths           []string   `json:"paths"`              // "folder/name" on any phone
	OnDeviceFileIDs []int64    `json:"on_device_file_ids"` // phone copies that can be deleted locally
	AccountID       *int64     `json:"account_id,omitempty"`
	ProductURL      *string    `json:"product_url,omitempty"`
	BackupStatus    string     `json:"backup_status"` // verified | uploaded | ""
	MarkedForDelete bool       `json:"marked_for_delete"`
}

func (s *Store) MediaDetails(ctx context.Context, userID int64, ids []int64) (map[int64]*MediaDetail, error) {
	out := map[int64]*MediaDetail{}
	rows, err := s.Pool.Query(ctx, `
		SELECT m.id, m.date_taken, m.width, m.height, b.account_id, b.product_url, COALESCE(b.status, ''),
		       b.marked_for_delete_at IS NOT NULL
		FROM media m
		LEFT JOIN backup_record b ON b.media_id = m.id AND b.status IN ('uploaded', 'verified')
		WHERE m.user_id = $1 AND m.id = ANY($2)`, userID, ids)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		d := &MediaDetail{Paths: []string{}, OnDeviceFileIDs: []int64{}}
		if err := rows.Scan(&d.MediaID, &d.DateTaken, &d.Width, &d.Height, &d.AccountID, &d.ProductURL, &d.BackupStatus, &d.MarkedForDelete); err != nil {
			rows.Close()
			return nil, err
		}
		out[d.MediaID] = d
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = s.Pool.Query(ctx, `
		SELECT f.media_id, f.id, f.relative_path || '/' || f.display_name, f.present_on_device
		FROM device_file f JOIN device d ON d.id = f.device_id
		WHERE d.user_id = $1 AND f.media_id = ANY($2) ORDER BY f.id`, userID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var mid, fid int64
		var path string
		var present bool
		if err := rows.Scan(&mid, &fid, &path, &present); err != nil {
			return nil, err
		}
		if d := out[mid]; d != nil {
			d.Paths = append(d.Paths, path)
			if present {
				d.OnDeviceFileIDs = append(d.OnDeviceFileIDs, fid)
			}
		}
	}
	return out, rows.Err()
}

// ToDeleteItem is a live backup chosen for removal from Google Photos.
type ToDeleteItem struct {
	MediaID     int64
	AccountID   int64
	MediaItemID string
}

// LiveBackups returns the live backups (with a Google item) of the given media.
func (s *Store) LiveBackups(ctx context.Context, userID int64, ids []int64) ([]ToDeleteItem, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT b.media_id, b.account_id, b.gphotos_media_item_id
		FROM backup_record b JOIN media m ON m.id = b.media_id
		WHERE m.user_id = $1 AND b.media_id = ANY($2) AND b.status IN ('uploaded', 'verified')
		  AND b.gphotos_media_item_id IS NOT NULL
		ORDER BY b.account_id, b.media_id`, userID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ToDeleteItem
	for rows.Next() {
		var it ToDeleteItem
		if err := rows.Scan(&it.MediaID, &it.AccountID, &it.MediaItemID); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) MarkForDelete(ctx context.Context, mediaIDs []int64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE backup_record SET marked_for_delete_at = now()
		WHERE media_id = ANY($1) AND status IN ('uploaded', 'verified') AND marked_for_delete_at IS NULL`, mediaIDs)
	return err
}

// ToDeleteFolder is the album.folder_path key of an account's "To delete" album (not a real phone folder).
const ToDeleteFolder = ":to-delete:"

// AlbumLink returns the Google Photos link of an album row.
func (s *Store) AlbumLink(ctx context.Context, accountID int64, folder string) (string, error) {
	var link *string
	err := s.Pool.QueryRow(ctx, `SELECT product_url FROM album WHERE account_id = $1 AND folder_path = $2`, accountID, folder).Scan(&link)
	if link == nil {
		return "", err
	}
	return *link, err
}

// PendingDeletes counts, per account, cloud copies marked for deletion that are still in Google.
func (s *Store) PendingDeletes(ctx context.Context, userID int64) (map[int64]int, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT b.account_id, count(*) FROM backup_record b JOIN account a ON a.id = b.account_id
		WHERE a.user_id = $1 AND b.marked_for_delete_at IS NOT NULL AND b.status IN ('uploaded', 'verified')
		GROUP BY b.account_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int{}
	for rows.Next() {
		var a int64
		var n int
		if err := rows.Scan(&a, &n); err != nil {
			return nil, err
		}
		out[a] = n
	}
	return out, rows.Err()
}
