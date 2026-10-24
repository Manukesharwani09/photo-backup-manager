package enum

// RuleBackup is the whether a folder rule backs files up. Values are stored as-is in the database.
type RuleBackup string

const (
	RULE_BACKUP_INCLUDE RuleBackup = "include"
	RULE_BACKUP_EXCLUDE RuleBackup = "exclude"
)

func (v RuleBackup) IsValid() bool {
	switch v {
	case RULE_BACKUP_INCLUDE,
		RULE_BACKUP_EXCLUDE:
		return true
	default:
		return false
	}
}
