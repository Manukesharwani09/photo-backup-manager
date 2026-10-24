package entity

import (
	"time"

	"github.com/Manukesharwani09/photo-backup-manager/internal/enum"
)

// AccountPeriod is a span during which an account was the owner's current backup destination.
type AccountPeriod struct {
	BaseModel
	AccountID int64                 `db:"account_id" json:"accountId"`
	StartedAt time.Time             `db:"started_at" json:"startedAt"`
	EndedAt   *time.Time            `db:"ended_at" json:"endedAt"`
	EndReason *enum.PeriodEndReason `db:"end_reason" json:"endReason"`
}
