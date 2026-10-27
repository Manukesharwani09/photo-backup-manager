package cleanup

type PlanRequest struct {
	DeviceID int64    `json:"device_id"`
	Folders  []string `json:"folders"` // each includes its sub-folders
}

// File is a phone file that is safe to delete.
type File struct {
	FileID       int64  `json:"file_id"`
	RelativePath string `json:"relative_path"`
	DisplayName  string `json:"display_name"`
	Size         int64  `json:"size"`
	AccountID    int64  `json:"account_id"`
	AccountEmail string `json:"account_email"`
	ProductURL   string `json:"product_url"`
}

type PlanResponse struct {
	Eligible   []File         `json:"eligible"`
	TotalBytes int64          `json:"total_bytes"`
	Kept       map[string]int `json:"kept"` // reason -> count, for the preview's explanation
}

type DoneRequest struct {
	DeviceID int64   `json:"device_id"`
	FileIDs  []int64 `json:"file_ids"`
}

type DoneResponse struct {
	MarkedRemoved int64 `json:"marked_removed"`
	Ignored       int64 `json:"ignored"`
}
