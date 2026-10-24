package enum

// AccountRole is the role of a linked Google account within an owner. Values are stored as-is in the database.
type AccountRole string

const (
	ACCOUNT_ROLE_PRIMARY  AccountRole = "primary"
	ACCOUNT_ROLE_OVERFLOW AccountRole = "overflow"
)

func (v AccountRole) IsValid() bool {
	switch v {
	case ACCOUNT_ROLE_PRIMARY,
		ACCOUNT_ROLE_OVERFLOW:
		return true
	default:
		return false
	}
}
