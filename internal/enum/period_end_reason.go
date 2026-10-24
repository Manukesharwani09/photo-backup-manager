package enum

// PeriodEndReason is the why an account stopped being the current backup destination. Values are stored as-is in the database.
type PeriodEndReason string

const (
	PERIOD_END_FULL     PeriodEndReason = "full"
	PERIOD_END_SWITCHED PeriodEndReason = "switched"
	PERIOD_END_MANUAL   PeriodEndReason = "manual"
)

func (v PeriodEndReason) IsValid() bool {
	switch v {
	case PERIOD_END_FULL,
		PERIOD_END_SWITCHED,
		PERIOD_END_MANUAL:
		return true
	default:
		return false
	}
}
