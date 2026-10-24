package enum

// FileSource is the where a phone file came from. Values are stored as-is in the database.
type FileSource string

const (
	FILE_SOURCE_CAMERA     FileSource = "camera"
	FILE_SOURCE_WHATSAPP   FileSource = "whatsapp"
	FILE_SOURCE_SCREENSHOT FileSource = "screenshot"
	FILE_SOURCE_DOWNLOAD   FileSource = "download"
	FILE_SOURCE_OTHER      FileSource = "other"
)

func (v FileSource) IsValid() bool {
	switch v {
	case FILE_SOURCE_CAMERA,
		FILE_SOURCE_WHATSAPP,
		FILE_SOURCE_SCREENSHOT,
		FILE_SOURCE_DOWNLOAD,
		FILE_SOURCE_OTHER:
		return true
	default:
		return false
	}
}
