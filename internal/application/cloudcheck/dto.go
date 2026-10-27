package cloudcheck

// Result summarises one pass over all live backups.
type Result struct {
	Checked        int      `json:"checked"`
	StillThere     int      `json:"still_there"`
	DeletedInCloud int      `json:"deleted_in_cloud"`
	Requeued       int64    `json:"requeued_for_upload"` // phone copies that will be uploaded again
	Errors         []string `json:"errors"`
}
