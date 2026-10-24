package entity

import (
	"time"

	"github.com/Manukesharwani09/photo-backup-manager/internal/enum"
)

// FolderRule decides, per phone folder, whether to back up, to which account, and whether cleanup may
// delete local copies. Replaced rule sets are soft-deleted (DeletedAt).
type FolderRule struct {
	BaseModel
	UserID          int64            `db:"user_id" json:"userId"`
	PathGlob        string           `db:"path_glob" json:"path"`
	TargetAccountID *int64           `db:"target_account_id" json:"targetAccountId"`
	Backup          enum.RuleBackup  `db:"backup" json:"backup"`
	Cleanup         enum.RuleCleanup `db:"cleanup" json:"cleanup"`
	Priority        int              `db:"priority" json:"priority"`
	DeletedAt       *time.Time       `db:"deleted_at" json:"-"`
}
