package appserver

import (
	"net/http"

	"github.com/Manukesharwani09/photo-backup-manager/internal/application"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/account"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/backup"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/cleanup"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/cloudcheck"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/dashboard"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/restore"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/rule"
	"github.com/Manukesharwani09/photo-backup-manager/internal/application/similar"
)

// NewRouter registers every route. This is the single list of the API; keep
// docs/architecture/overview.md#backend-api in sync with it.
func NewRouter(svcs application.Services) http.Handler {
	accountCtrl := account.NewAccountController(svcs.Account)
	backupCtrl := backup.NewBackupController(svcs.Backup)
	ruleCtrl := rule.NewRuleController(svcs.Rule)
	cleanupCtrl := cleanup.NewCleanupController(svcs.Cleanup)
	restoreCtrl := restore.NewRestoreController(svcs.Restore)
	similarCtrl := similar.NewSimilarController(svcs.Similar)
	cloudCheckCtrl := cloudcheck.NewCloudCheckController(svcs.CloudCheck)
	dashboardCtrl := dashboard.NewDashboardController(svcs.Dashboard)

	mux := http.NewServeMux()
	// Google sign-in. Not versioned: /oauth/callback is the redirect URI registered with Google.
	mux.HandleFunc("GET /oauth/start", handle(accountCtrl.OAuthStart))
	mux.HandleFunc("GET /oauth/callback", handle(accountCtrl.OAuthCallback))

	mux.HandleFunc("GET /v1/accounts", handle(accountCtrl.List))
	mux.HandleFunc("POST /v1/accounts/{id}/full", handle(accountCtrl.MarkFull))
	mux.HandleFunc("POST /v1/accounts/{id}/available", handle(accountCtrl.MarkAvailable))
	mux.HandleFunc("POST /v1/accounts/{id}/current", handle(accountCtrl.MakeCurrent))

	mux.HandleFunc("POST /v1/devices", handle(backupCtrl.CreateDevice))
	mux.HandleFunc("POST /v1/sync", handle(backupCtrl.Sync))
	mux.HandleFunc("POST /v1/uploads", handle(backupCtrl.CompleteUploads))

	mux.HandleFunc("GET /v1/rules", handle(ruleCtrl.List))
	mux.HandleFunc("PUT /v1/rules", handle(ruleCtrl.Replace))

	mux.HandleFunc("POST /v1/cleanup/plan", handle(cleanupCtrl.Plan))
	mux.HandleFunc("POST /v1/cleanup/done", handle(cleanupCtrl.Done))

	mux.HandleFunc("POST /v1/restore/plan", handle(restoreCtrl.Plan))
	mux.HandleFunc("POST /v1/restore/done", handle(restoreCtrl.Done))

	mux.HandleFunc("GET /v1/similar", handle(similarCtrl.Groups))
	mux.HandleFunc("POST /v1/cloud/to-delete", handle(similarCtrl.MarkToDelete))
	mux.HandleFunc("POST /v1/cloud/check", handle(cloudCheckCtrl.Check))

	mux.HandleFunc("GET /v1/dashboard", handle(dashboardCtrl.Dashboard))
	mux.HandleFunc("GET /v1/photos/where", handle(dashboardCtrl.Where))

	return withRequestContext(mux)
}
