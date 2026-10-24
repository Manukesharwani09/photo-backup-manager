package enum

// FileState is the backup state of a file on a phone (device_file.state). Values are stored as-is in the database.
type FileState string

const (
	FILE_STATE_EXISTING_SKIP FileState = "existing_skip"
	FILE_STATE_PENDING       FileState = "pending"
	FILE_STATE_BACKED_UP     FileState = "backed_up"
	FILE_STATE_EXCLUDED      FileState = "excluded"
)

func (v FileState) IsValid() bool {
	switch v {
	case FILE_STATE_EXISTING_SKIP,
		FILE_STATE_PENDING,
		FILE_STATE_BACKED_UP,
		FILE_STATE_EXCLUDED:
		return true
	default:
		return false
	}
}
