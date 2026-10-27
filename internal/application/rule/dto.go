package rule

// RuleDTO is one folder rule as the app sees it.
type RuleDTO struct {
	PathGlob        string `json:"path"`
	TargetAccountID *int64 `json:"target_account_id"`
	Backup          string `json:"backup"`  // include | exclude (default include)
	Cleanup         string `json:"cleanup"` // allowed | protected (default allowed)
	Priority        int    `json:"priority"`
}
