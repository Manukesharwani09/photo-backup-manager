package entity

import "time"

// Media is a unique photo/video content, identified by the SHA-256 of its bytes.
// The same bytes in several folders or phones are one Media.
type Media struct {
	BaseModel
	UserID    int64      `db:"user_id" json:"userId"`
	SHA256    []byte     `db:"sha256" json:"-"`
	PHash     *int64     `db:"phash" json:"-"` // 64-bit perceptual hash, stored as its signed bit pattern
	Width     *int       `db:"width" json:"width"`
	Height    *int       `db:"height" json:"height"`
	DateTaken *time.Time `db:"date_taken" json:"dateTaken"`
	GPSLat    *float64   `db:"gps_lat" json:"gpsLat"`
	GPSLng    *float64   `db:"gps_lng" json:"gpsLng"`
}
