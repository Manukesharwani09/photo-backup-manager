package repository

import "context"

// LiveRecord is a live backup to re-check against Google.
type LiveRecord struct {
	ID              int64
	MediaID         int64
	MediaItemID     string
	MarkedForDelete bool
}

// LiveRecordsByAccount groups the user's live backups (with a Google item) by account.
func (s *Store) LiveRecordsByAccount(ctx context.Context, userID int64) (map[int64][]LiveRecord, []int64, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT b.account_id, b.id, b.media_id, b.gphotos_media_item_id, b.marked_for_delete_at IS NOT NULL
		FROM backup_record b JOIN account a ON a.id = b.account_id
		WHERE a.user_id = $1 AND b.status IN ('uploaded', 'verified') AND b.gphotos_media_item_id IS NOT NULL
		ORDER BY b.account_id, b.id`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := map[int64][]LiveRecord{}
	var order []int64
	for rows.Next() {
		var acct int64
		var r LiveRecord
		if err := rows.Scan(&acct, &r.ID, &r.MediaID, &r.MediaItemID, &r.MarkedForDelete); err != nil {
			return nil, nil, err
		}
		if out[acct] == nil {
			order = append(order, acct)
		}
		out[acct] = append(out[acct], r)
	}
	return out, order, rows.Err()
}

// MarkDeletedInCloud records that Google no longer has a backup's item. The photo stops counting as
// backed up. Phone copies the user deliberately sent to "To delete" become excluded (never re-uploaded);
// otherwise copies still on a phone go back to pending so the next sync uploads them again.
// Returns how many phone files were re-queued.
func (s *Store) MarkDeletedInCloud(ctx context.Context, r LiveRecord) (requeued int64, err error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE backup_record SET status = 'deleted_in_cloud', last_checked_at = now() WHERE id = $1`, r.ID); err != nil {
		return 0, err
	}
	newState := "pending"
	if r.MarkedForDelete {
		newState = "excluded"
	}
	tag, err := tx.Exec(ctx, `
		UPDATE device_file SET state = $2
		WHERE media_id = $1 AND present_on_device AND state = 'backed_up'
		  AND NOT EXISTS (SELECT 1 FROM backup_record b WHERE b.media_id = $1 AND b.status IN ('uploaded', 'verified'))`,
		r.MediaID, newState)
	if err != nil {
		return 0, err
	}
	if newState == "pending" {
		requeued = tag.RowsAffected()
	}
	return requeued, tx.Commit(ctx)
}

// TouchChecked records that these backups were confirmed present.
func (s *Store) TouchChecked(ctx context.Context, recordIDs []int64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE backup_record SET last_checked_at = now(),
		status = 'verified', verified_at = COALESCE(verified_at, now()) WHERE id = ANY($1)`, recordIDs)
	return err
}
