package enum

import "testing"

func TestEnums_IsValid(t *testing.T) {
	valid := []interface{ IsValid() bool }{
		ACCOUNT_ROLE_PRIMARY, ACCOUNT_ROLE_OVERFLOW,
		FILE_STATE_EXISTING_SKIP, FILE_STATE_PENDING, FILE_STATE_BACKED_UP, FILE_STATE_EXCLUDED,
		BACKUP_STATUS_UPLOADED, BACKUP_STATUS_VERIFIED, BACKUP_STATUS_FAILED, BACKUP_STATUS_DELETED_IN_CLOUD,
		FILE_SOURCE_CAMERA, FILE_SOURCE_WHATSAPP, FILE_SOURCE_SCREENSHOT, FILE_SOURCE_DOWNLOAD, FILE_SOURCE_OTHER,
		RULE_BACKUP_INCLUDE, RULE_BACKUP_EXCLUDE, RULE_CLEANUP_ALLOWED, RULE_CLEANUP_PROTECTED,
		PERIOD_END_FULL, PERIOD_END_SWITCHED, PERIOD_END_MANUAL,
	}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("%v should be valid", v)
		}
	}
	invalid := []interface{ IsValid() bool }{
		AccountRole("admin"), FileState(""), BackupStatus("gone"), FileSource("telepathy"),
		RuleBackup("maybe"), RuleCleanup(""), PeriodEndReason("bored"),
	}
	for _, v := range invalid {
		if v.IsValid() {
			t.Errorf("%v should be invalid", v)
		}
	}
}
