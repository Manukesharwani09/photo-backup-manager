package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Manukesharwani09/photo-backup-manager/internal/plan"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyBacked = errors.New("media already has a live backup")
)

// FileMeta is what the phone reports for one file.
type FileMeta struct {
	RelativePath string
	DisplayName  string
	Size         int64
	SHA256       []byte
	DateTaken    *time.Time
	Width        *int
	Height       *int
	GPSLat       *float64
	GPSLng       *float64
	PHash        *int64 // 64-bit perceptual hash stored as its signed bit pattern
	Source       string
}

func (s *Store) CreateDevice(ctx context.Context, userID int64, model string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO device (user_id, model) VALUES ($1, $2) RETURNING id`, userID, model).Scan(&id)
	return id, err
}

// DeviceFirstScan returns when the device's first scan happened (nil = not yet). ErrNotFound if it isn't the user's.
func (s *Store) DeviceFirstScan(ctx context.Context, userID, deviceID int64) (*time.Time, error) {
	var at *time.Time
	err := s.Pool.QueryRow(ctx, `SELECT first_scan_at FROM device WHERE id = $1 AND user_id = $2`, deviceID, userID).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return at, err
}

func (s *Store) MarkFirstScan(ctx context.Context, deviceID int64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE device SET first_scan_at = now() WHERE id = $1 AND first_scan_at IS NULL`, deviceID)
	return err
}

// UpsertFile records a phone file and its content. It returns the file id, its media id, the state it had
// before this call ("" if new) and whether its content (media) changed since the last report.
func (s *Store) UpsertFile(ctx context.Context, userID, deviceID int64, f FileMeta) (fileID, mediaID int64, prevState string, contentChanged bool, err error) {
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO media (user_id, sha256, width, height, date_taken, gps_lat, gps_lng, phash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, sha256) DO UPDATE SET
		  phash = COALESCE(media.phash, EXCLUDED.phash),
		  width = COALESCE(media.width, EXCLUDED.width),
		  height = COALESCE(media.height, EXCLUDED.height),
		  date_taken = COALESCE(media.date_taken, EXCLUDED.date_taken),
		  gps_lat = COALESCE(media.gps_lat, EXCLUDED.gps_lat),
		  gps_lng = COALESCE(media.gps_lng, EXCLUDED.gps_lng)
		RETURNING id`,
		userID, f.SHA256, f.Width, f.Height, f.DateTaken, f.GPSLat, f.GPSLng, f.PHash).Scan(&mediaID)
	if err != nil {
		return
	}
	var prevMedia *int64
	var prev *string
	err = s.Pool.QueryRow(ctx, `SELECT media_id, state FROM device_file WHERE device_id = $1 AND relative_path = $2 AND display_name = $3`,
		deviceID, f.RelativePath, f.DisplayName).Scan(&prevMedia, &prev)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if prev != nil {
		prevState = *prev
		contentChanged = *prevMedia != mediaID
	}
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO device_file (device_id, media_id, relative_path, display_name, size, source, state)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')
		ON CONFLICT (device_id, relative_path, display_name) DO UPDATE SET
		  media_id = EXCLUDED.media_id, size = EXCLUDED.size, source = EXCLUDED.source, present_on_device = TRUE
		RETURNING id`,
		deviceID, mediaID, f.RelativePath, f.DisplayName, f.Size, f.Source).Scan(&fileID)
	return
}

func (s *Store) SetFileState(ctx context.Context, fileID int64, state string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE device_file SET state = $2 WHERE id = $1`, fileID, state)
	return err
}

// IsBackedUp reports whether the media has a live backup in any account.
func (s *Store) IsBackedUp(ctx context.Context, mediaID int64) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM backup_record WHERE media_id = $1 AND status IN ('uploaded','verified'))`, mediaID).Scan(&ok)
	return ok, err
}

// DeletedOnPurpose reports whether the user sent this media's cloud copy to "To delete" and then
// deleted it in Google Photos. Such a photo must never be uploaded again.
func (s *Store) DeletedOnPurpose(ctx context.Context, mediaID int64) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM backup_record
		WHERE media_id = $1 AND status = 'deleted_in_cloud' AND marked_for_delete_at IS NOT NULL)`, mediaID).Scan(&ok)
	return ok, err
}

// UploadFile is a device file the phone says it uploaded.
type UploadFile struct {
	ID           int64
	MediaID      int64
	RelativePath string
	DisplayName  string
	State        string
}

func (s *Store) FileOnDevice(ctx context.Context, deviceID, fileID int64) (UploadFile, error) {
	var f UploadFile
	err := s.Pool.QueryRow(ctx, `SELECT id, media_id, relative_path, display_name, state FROM device_file WHERE id = $1 AND device_id = $2`,
		fileID, deviceID).Scan(&f.ID, &f.MediaID, &f.RelativePath, &f.DisplayName, &f.State)
	if errors.Is(err, pgx.ErrNoRows) {
		return f, ErrNotFound
	}
	return f, err
}

func (s *Store) ListRules(ctx context.Context, userID int64) ([]plan.Rule, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, path_glob, target_account_id, backup, cleanup, priority FROM folder_rule WHERE user_id = $1 ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []plan.Rule
	for rows.Next() {
		var r plan.Rule
		if err := rows.Scan(&r.ID, &r.PathGlob, &r.TargetAccountID, &r.Backup, &r.Cleanup, &r.Priority); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReplaceRules swaps the user's whole rule set atomically. Target accounts must belong to the user.
func (s *Store) ReplaceRules(ctx context.Context, userID int64, rules []plan.Rule) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM folder_rule WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, r := range rules {
		if r.TargetAccountID != nil {
			var ok bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM account WHERE id = $1 AND user_id = $2)`, *r.TargetAccountID, userID).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				return ErrNotFound
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO folder_rule (user_id, path_glob, target_account_id, backup, cleanup, priority) VALUES ($1,$2,$3,$4,$5,$6)`,
			userID, plan.NormalizeFolder(r.PathGlob), r.TargetAccountID, r.Backup, r.Cleanup, r.Priority)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// PlannerAccounts returns the user's accounts with which one is current (has the open period).
func (s *Store) PlannerAccounts(ctx context.Context, userID int64) ([]plan.Account, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT a.id, a.role, a.active, a.full_flag,
		       EXISTS (SELECT 1 FROM account_period p WHERE p.account_id = a.id AND p.ended_at IS NULL)
		FROM account a WHERE a.user_id = $1 ORDER BY a.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []plan.Account
	for rows.Next() {
		var a plan.Account
		if err := rows.Scan(&a.ID, &a.Role, &a.Active, &a.Full, &a.IsCurrent); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SwitchCurrent closes the user's open period with reason and opens one for toAccountID.
func (s *Store) SwitchCurrent(ctx context.Context, userID, toAccountID int64, reason string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		UPDATE account_period SET ended_at = now(), end_reason = $2
		WHERE ended_at IS NULL AND account_id IN (SELECT id FROM account WHERE user_id = $1)`, userID, reason)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO account_period (account_id) VALUES ($1)`, toAccountID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// MarkFull flags an account as out of storage. ErrNotFound if it isn't the user's.
func (s *Store) MarkFull(ctx context.Context, userID, accountID int64) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE account SET full_flag = TRUE WHERE id = $1 AND user_id = $2`, accountID, userID)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// AccountOwned returns the account if it belongs to the user.
func (s *Store) AccountOwned(ctx context.Context, userID, accountID int64) (Account, error) {
	var a Account
	err := s.Pool.QueryRow(ctx, `SELECT id, email, role, active, full_flag, refresh_token_enc FROM account WHERE id = $1 AND user_id = $2`,
		accountID, userID).Scan(&a.ID, &a.Email, &a.Role, &a.Active, &a.FullFlag, &a.RefreshTokenEnc)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	return a, err
}

// Album returns the Google album id and row id for a folder in an account, if created before.
func (s *Store) Album(ctx context.Context, accountID int64, folder string) (rowID int64, googleID string, err error) {
	err = s.Pool.QueryRow(ctx, `SELECT id, gphotos_album_id FROM album WHERE account_id = $1 AND folder_path = $2`, accountID, folder).Scan(&rowID, &googleID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return
}

func (s *Store) SaveAlbum(ctx context.Context, accountID int64, folder, title, googleID, productURL string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO album (account_id, folder_path, title, gphotos_album_id, product_url) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (account_id, folder_path) DO UPDATE SET gphotos_album_id = EXCLUDED.gphotos_album_id, product_url = EXCLUDED.product_url
		RETURNING id`, accountID, folder, title, googleID, productURL).Scan(&id)
	return id, err
}

// RecordBackup stores a successful upload and marks every device copy of that media as backed up.
// ErrAlreadyBacked if another live backup exists (the unique index enforces this).
func (s *Store) RecordBackup(ctx context.Context, mediaID, accountID int64, itemID, productURL string, albumRowID *int64) (int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO backup_record (media_id, account_id, gphotos_media_item_id, product_url, album_id, status)
		VALUES ($1,$2,$3,$4,$5,'uploaded') RETURNING id`, mediaID, accountID, itemID, productURL, albumRowID).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return 0, ErrAlreadyBacked
	}
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `UPDATE device_file SET state = 'backed_up' WHERE media_id = $1`, mediaID); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

func (s *Store) MarkVerified(ctx context.Context, recordIDs []int64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE backup_record SET status = 'verified', verified_at = now(), last_checked_at = now() WHERE id = ANY($1)`, recordIDs)
	return err
}
