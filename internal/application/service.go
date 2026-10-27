// Package application wires the feature services together. Each feature lives in its own package with
// controller.go (HTTP only), service.go (business logic) and dto.go (request/response shapes).
package application

import (
	"golang.org/x/oauth2"

	"github.com/Manukesharwani09/photo-backup-manager/internal/application/account"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/backup"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/cleanup"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/cloudcheck"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/dashboard"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/restore"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/rule"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/similar"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
)

// Deps are what the services need from the outside world.
type Deps struct {
	Repo     *repository.Store
	OAuth    *oauth2.Config
	TokenKey []byte
	UserID   int64 // the single local owner until app ↔ backend sign-in exists
}

type Services struct {
	Account    account.AccountService
	Backup     backup.BackupService
	Rule       rule.RuleService
	Cleanup    cleanup.CleanupService
	Restore    restore.RestoreService
	Similar    similar.SimilarService
	CloudCheck cloudcheck.CloudCheckService
	Dashboard  dashboard.DashboardService
}

// NewServices is the one place services are created and given their dependencies.
func NewServices(d Deps) Services {
	access := account.NewAccess(d.Repo, d.OAuth, d.TokenKey, d.UserID)
	return Services{
		Account:    account.NewAccountService(d.Repo, d.OAuth, access, d.TokenKey, d.UserID),
		Backup:     backup.NewBackupService(d.Repo, access, d.UserID),
		Rule:       rule.NewRuleService(d.Repo, d.UserID),
		Cleanup:    cleanup.NewCleanupService(d.Repo, d.UserID),
		Restore:    restore.NewRestoreService(d.Repo, access, d.UserID),
		Similar:    similar.NewSimilarService(d.Repo, access, d.UserID),
		CloudCheck: cloudcheck.NewCloudCheckService(d.Repo, access, d.UserID),
		Dashboard:  dashboard.NewDashboardService(d.Repo, access, d.UserID),
	}
}
