package account

// Period is one active period of an account, as shown to the app.
type Period struct {
	StartedAt string  `json:"started_at"`
	EndedAt   *string `json:"ended_at"`
	EndReason *string `json:"end_reason"`
}

// AccountResponse is one account in GET /v1/accounts.
type AccountResponse struct {
	ID            int64    `json:"id"`
	Email         string   `json:"email"`
	Role          string   `json:"role"`
	Active        bool     `json:"active"`
	Full          bool     `json:"full"`
	StorageLimit  int64    `json:"storage_limit_bytes"`
	StorageUsed   int64    `json:"storage_used_bytes"`
	StorageFree   int64    `json:"storage_free_bytes"` // -1 = unlimited
	StorageFreeGB string   `json:"storage_free"`
	StorageError  string   `json:"storage_error,omitempty"`
	ActivePeriods []Period `json:"active_periods"`
}

// ConnectedResponse describes a successfully linked Google account.
type ConnectedResponse struct {
	AccountID int64
	Email     string
	Role      string
}

// MarkFullResponse is the body of POST /v1/accounts/{id}/full.
type MarkFullResponse struct {
	AccountID int64 `json:"account_id"`
	Full      bool  `json:"full"`
}
