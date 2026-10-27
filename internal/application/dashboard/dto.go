package dashboard

import (
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/account"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
)

// Alert kinds shown on the Home screen.
const (
	ALERT_ACCOUNT_FULL         = "account_full"
	ALERT_NO_ACCOUNT_AVAILABLE = "no_account_available"
	ALERT_TO_DELETE_WAITING    = "to_delete_waiting"
	ALERT_ACCOUNT_UNREACHABLE  = "account_unreachable"
)

type Alert struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

type AccountSummary struct {
	ID            int64            `json:"id"`
	Email         string           `json:"email"`
	Role          string           `json:"role"`
	Active        bool             `json:"active"`
	Full          bool             `json:"full"`
	Current       bool             `json:"current"` // receiving new backups now
	StorageFree   string           `json:"storage_free,omitempty"`
	StorageError  string           `json:"storage_error,omitempty"`
	ActivePeriods []account.Period `json:"active_periods"`
	ToDeleteAlbum string           `json:"to_delete_album_url,omitempty"`
	*repository.AccountStats
}

type DashboardResponse struct {
	Accounts []AccountSummary `json:"accounts"`
	Phone    map[string]int   `json:"phone_files"` // by state, files still on a phone
	Alerts   []Alert          `json:"alerts"`
}

type WhereResponse struct {
	From           string               `json:"from"`
	ToExclusive    string               `json:"to_exclusive"`
	Accounts       []repository.TakenIn `json:"accounts"`
	UndatedBackups int                  `json:"undated_backups"`
}
