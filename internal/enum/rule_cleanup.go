package enum

// RuleCleanup is the whether cleanup may delete a folder's phone files. Values are stored as-is in the database.
type RuleCleanup string

const (
	RULE_CLEANUP_ALLOWED   RuleCleanup = "allowed"
	RULE_CLEANUP_PROTECTED RuleCleanup = "protected"
)

func (v RuleCleanup) IsValid() bool {
	switch v {
	case RULE_CLEANUP_ALLOWED,
		RULE_CLEANUP_PROTECTED:
		return true
	default:
		return false
	}
}
