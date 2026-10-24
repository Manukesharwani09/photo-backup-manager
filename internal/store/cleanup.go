package store

import "context"

// DeviceFileStatus is a phone file with its live backup (if any), for cleanup and restore decisions.
type DeviceFileStatus struct {
	ID           int64
	MediaID      int64
	RelativePath string
	DisplayName  string
	Size         int64
	State        string
	OnDevice     bool
	BackupStatus string // verified | uploaded | "" (no live backup)
	AccountID    *int64
	AccountEmail *string
	MediaItemID  *string
	ProductURL   *string
}

// DeviceFiles lists every file the device has reported, with its live backup.
func (s *Store) DeviceFiles(ctx context.Context, deviceID int64) ([]DeviceFileStatus, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT f.id, f.media_id, f.relative_path, f.display_name, f.size, f.state, f.present_on_device,
		       COALESCE(b.status, ''), b.account_id, a.email, b.gphotos_media_item_id, b.product_url
		FROM device_file f
		LEFT JOIN backup_record b ON b.media_id = f.media_id AND b.status IN ('uploaded', 'verified')
		LEFT JOIN account a ON a.id = b.account_id
		WHERE f.device_id = $1
		ORDER BY f.relative_path, f.display_name`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeviceFileStatus
	for rows.Next() {
		var f DeviceFileStatus
		if err := rows.Scan(&f.ID, &f.MediaID, &f.RelativePath, &f.DisplayName, &f.Size, &f.State, &f.OnDevice,
			&f.BackupStatus, &f.AccountID, &f.AccountEmail, &f.MediaItemID, &f.ProductURL); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// MarkRemovedFromDevice records that the phone deleted these files. Only files with a verified
// backup are touched, so a buggy client can't mark an unprotected photo as safely gone.
func (s *Store) MarkRemovedFromDevice(ctx context.Context, deviceID int64, fileIDs []int64) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE device_file f SET present_on_device = FALSE
		WHERE f.device_id = $1 AND f.id = ANY($2) AND f.present_on_device
		  AND EXISTS (SELECT 1 FROM backup_record b WHERE b.media_id = f.media_id AND b.status = 'verified')`,
		deviceID, fileIDs)
	return tag.RowsAffected(), err
}
