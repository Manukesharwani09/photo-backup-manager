package entity

import "github.com/Manukesharwani09/photo-backup-manager/internal/enum"

// Account is a Google account linked to an owner: exactly one primary, any number of overflow.
// A Google account belongs to one owner only (unique google_sub and lower(email)).
type Account struct {
	BaseModel
	UserID          int64            `db:"user_id" json:"userId"`
	Email           string           `db:"email" json:"email"`
	GoogleSub       *string          `db:"google_sub" json:"-"`
	Role            enum.AccountRole `db:"role" json:"role"`
	Active          bool             `db:"active" json:"active"`
	FullFlag        bool             `db:"full_flag" json:"full"`
	RefreshTokenEnc []byte           `db:"refresh_token_enc" json:"-"` // AES-256-GCM, see infrastructure/crypto
}
