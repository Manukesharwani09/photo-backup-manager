package store

import (
	"context"
	"time"
)

// AccountStats is what the dashboard shows per account about the photos it holds.
type AccountStats struct {
	AccountID     int64      `json:"-"`
	Items         int        `json:"items"`
	TakenFrom     *time.Time `json:"photos_taken_from,omitempty"`
	TakenTo       *time.Time `json:"photos_taken_to,omitempty"`
	LastUpload    *time.Time `json:"last_upload,omitempty"`
	Folders       []string   `json:"folders"`
	PendingDelete int        `json:"waiting_in_to_delete"`
}

// AccountStats aggregates live backups per account: count, photo-date range, last upload, folders.
// Counts are of distinct Google items: a restored file is linked to the same item as its original.
func (s *Store) AccountStats(ctx context.Context, userID int64) (map[int64]*AccountStats, error) {
	out := map[int64]*AccountStats{}
	rows, err := s.Pool.Query(ctx, `
		SELECT b.account_id, count(DISTINCT b.gphotos_media_item_id), min(m.date_taken), max(m.date_taken), max(b.uploaded_at),
		       count(DISTINCT b.gphotos_media_item_id) FILTER (WHERE b.marked_for_delete_at IS NOT NULL)
		FROM backup_record b JOIN media m ON m.id = b.media_id JOIN account a ON a.id = b.account_id
		WHERE a.user_id = $1 AND b.status IN ('uploaded', 'verified')
		GROUP BY b.account_id`, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		st := &AccountStats{Folders: []string{}}
		if err := rows.Scan(&st.AccountID, &st.Items, &st.TakenFrom, &st.TakenTo, &st.LastUpload, &st.PendingDelete); err != nil {
			rows.Close()
			return nil, err
		}
		out[st.AccountID] = st
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = s.Pool.Query(ctx, `
		SELECT DISTINCT b.account_id, f.relative_path
		FROM backup_record b JOIN device_file f ON f.media_id = b.media_id JOIN account a ON a.id = b.account_id
		WHERE a.user_id = $1 AND b.status IN ('uploaded', 'verified')
		ORDER BY 1, 2`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var acct int64
		var folder string
		if err := rows.Scan(&acct, &folder); err != nil {
			return nil, err
		}
		if st := out[acct]; st != nil {
			st.Folders = append(st.Folders, folder)
		}
	}
	return out, rows.Err()
}

// StateCounts counts phone files by state across the user's devices (only files still on a phone).
func (s *Store) StateCounts(ctx context.Context, userID int64) (map[string]int, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT f.state, count(*) FROM device_file f JOIN device d ON d.id = f.device_id
		WHERE d.user_id = $1 AND f.present_on_device GROUP BY f.state`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{"existing_skip": 0, "pending": 0, "backed_up": 0, "excluded": 0}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[st] = n
	}
	return out, rows.Err()
}

// TakenIn is, for one account, the backed-up photos taken in a period.
type TakenIn struct {
	AccountID    int64    `json:"account_id"`
	AccountEmail string   `json:"account_email"`
	Items        int      `json:"items"`
	Folders      []string `json:"folders"`
}

// WhereTaken answers "which accounts hold my photos taken between from and to?" (to exclusive).
func (s *Store) WhereTaken(ctx context.Context, userID int64, from, to time.Time) ([]TakenIn, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT a.id, a.email, count(DISTINCT b.gphotos_media_item_id),
		       COALESCE(array_agg(DISTINCT f.relative_path) FILTER (WHERE f.relative_path IS NOT NULL), '{}')
		FROM backup_record b
		JOIN media m ON m.id = b.media_id
		JOIN account a ON a.id = b.account_id
		LEFT JOIN device_file f ON f.media_id = m.id
		WHERE a.user_id = $1 AND b.status IN ('uploaded', 'verified')
		  AND m.date_taken >= $2 AND m.date_taken < $3
		GROUP BY a.id, a.email ORDER BY a.id`, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TakenIn{}
	for rows.Next() {
		var t TakenIn
		if err := rows.Scan(&t.AccountID, &t.AccountEmail, &t.Items, &t.Folders); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// UndatedBackups counts live backups without a date taken (they can't be found by date).
func (s *Store) UndatedBackups(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT count(*) FROM backup_record b JOIN media m ON m.id = b.media_id
		WHERE m.user_id = $1 AND b.status IN ('uploaded', 'verified') AND m.date_taken IS NULL`, userID).Scan(&n)
	return n, err
}
