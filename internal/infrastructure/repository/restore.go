package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// RestoreCandidate is one backed-up photo at one original phone path.
type RestoreCandidate struct {
	MediaID      int64
	RelativePath string
	DisplayName  string
	Size         int64
	DateTaken    *time.Time
	GPSLat       *float64
	GPSLng       *float64
	AccountID    int64
	MediaItemID  string
}

// RestoreCandidates lists every (photo, original path) with a live backup, across all of the user's
// devices, so a new phone can restore what an old phone backed up. A photo saved in two folders
// appears once per folder.
func (s *Store) RestoreCandidates(ctx context.Context, userID int64) ([]RestoreCandidate, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT DISTINCT ON (f.relative_path, f.display_name)
		       m.id, f.relative_path, f.display_name, f.size, m.date_taken, m.gps_lat, m.gps_lng,
		       b.account_id, b.gphotos_media_item_id
		FROM device_file f
		JOIN device d ON d.id = f.device_id AND d.user_id = $1
		JOIN media m ON m.id = f.media_id
		JOIN backup_record b ON b.media_id = m.id AND b.status IN ('uploaded', 'verified')
		WHERE b.gphotos_media_item_id IS NOT NULL
		ORDER BY f.relative_path, f.display_name, f.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RestoreCandidate
	for rows.Next() {
		var c RestoreCandidate
		if err := rows.Scan(&c.MediaID, &c.RelativePath, &c.DisplayName, &c.Size, &c.DateTaken, &c.GPSLat, &c.GPSLng,
			&c.AccountID, &c.MediaItemID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// PresentPaths returns "folder/name" of files the device currently has.
func (s *Store) PresentPaths(ctx context.Context, deviceID int64) (map[string]bool, error) {
	rows, err := s.Pool.Query(ctx, `SELECT relative_path || '/' || display_name FROM device_file WHERE device_id = $1 AND present_on_device`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out[p] = true
	}
	return out, rows.Err()
}

// RecordRestored stores a file the phone downloaded back from Google. Downloads may not be
// byte-identical to the original (Google can rewrite metadata), so the restored bytes get their own
// media row linked to the same Google item: the photo then counts as backed up and is never re-uploaded.
// Returns false if originalMediaID has no live backup for this user.
func (s *Store) RecordRestored(ctx context.Context, userID, deviceID, originalMediaID int64, f FileMeta) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var acct int64
	var item, url *string
	var album *int64
	var lat, lng *float64
	var taken *time.Time
	err = tx.QueryRow(ctx, `
		SELECT b.account_id, b.gphotos_media_item_id, b.product_url, b.album_id, m.gps_lat, m.gps_lng, m.date_taken
		FROM backup_record b JOIN media m ON m.id = b.media_id
		WHERE b.media_id = $1 AND m.user_id = $2 AND b.status IN ('uploaded', 'verified')`,
		originalMediaID, userID).Scan(&acct, &item, &url, &album, &lat, &lng, &taken)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // no live backup: nothing to link
	}
	if err != nil {
		return false, err
	}

	var mediaID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO media (user_id, sha256, width, height, date_taken, gps_lat, gps_lng)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, sha256) DO UPDATE SET user_id = EXCLUDED.user_id
		RETURNING id`, userID, f.SHA256, f.Width, f.Height, taken, lat, lng).Scan(&mediaID)
	if err != nil {
		return false, err
	}
	if mediaID != originalMediaID {
		_, err = tx.Exec(ctx, `
			INSERT INTO backup_record (media_id, account_id, gphotos_media_item_id, product_url, album_id, status, verified_at)
			SELECT $1, $2, $3, $4, $5, 'verified', now()
			WHERE NOT EXISTS (SELECT 1 FROM backup_record WHERE media_id = $1 AND status IN ('uploaded', 'verified'))`,
			mediaID, acct, item, url, album)
		if err != nil {
			return false, err
		}
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO device_file (device_id, media_id, relative_path, display_name, size, source, state, present_on_device)
		VALUES ($1, $2, $3, $4, $5, $6, 'backed_up', TRUE)
		ON CONFLICT (device_id, relative_path, display_name) DO UPDATE SET
		  media_id = EXCLUDED.media_id, size = EXCLUDED.size, state = 'backed_up', present_on_device = TRUE`,
		deviceID, mediaID, f.RelativePath, f.DisplayName, f.Size, f.Source)
	if err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
