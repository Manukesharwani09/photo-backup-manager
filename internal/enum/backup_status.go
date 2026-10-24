package enum

// BackupStatus is the status of a backup_record. Values are stored as-is in the database.
type BackupStatus string

const (
	BACKUP_STATUS_UPLOADED         BackupStatus = "uploaded"
	BACKUP_STATUS_VERIFIED         BackupStatus = "verified"
	BACKUP_STATUS_FAILED           BackupStatus = "failed"
	BACKUP_STATUS_DELETED_IN_CLOUD BackupStatus = "deleted_in_cloud"
)

func (v BackupStatus) IsValid() bool {
	switch v {
	case BACKUP_STATUS_UPLOADED,
		BACKUP_STATUS_VERIFIED,
		BACKUP_STATUS_FAILED,
		BACKUP_STATUS_DELETED_IN_CLOUD:
		return true
	default:
		return false
	}
}
