package entity

import "time"

// Device is a phone. FirstScanAt is its start point: files present then are never uploaded.
type Device struct {
	BaseModel
	UserID       int64      `db:"user_id" json:"userId"`
	Model        string     `db:"model" json:"model"`
	RegisteredAt time.Time  `db:"registered_at" json:"registeredAt"`
	FirstScanAt  *time.Time `db:"first_scan_at" json:"firstScanAt"`
}
