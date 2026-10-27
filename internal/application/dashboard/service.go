package dashboard

import (
	"context"
	"net/url"
	"time"

	"github.com/Manukesharwani09/photo-backup-manager/internal/application/account"
	"github.com/Manukesharwani09/photo-backup-manager/internal/common"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/httperror"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
	"github.com/Manukesharwani09/photo-backup-manager/internal/planner"
)

type DashboardService interface {
	// Dashboard is the Home/Accounts screen: per account what it holds and when it was used,
	// phone file counts by state, and alerts that need the user.
	Dashboard(ctx context.Context) (DashboardResponse, error)
	// Where answers "which account has my photos from <period>?": month=YYYY-MM or from=&to= (to inclusive).
	Where(ctx context.Context, q url.Values) (WhereResponse, error)
}

type dashboardService struct {
	repo   *repository.Store
	access *account.Access
	userID int64
}

func NewDashboardService(repo *repository.Store, access *account.Access, userID int64) DashboardService {
	return &dashboardService{repo: repo, access: access, userID: userID}
}

func (s *dashboardService) Dashboard(ctx context.Context) (DashboardResponse, error) {
	accts, err := s.repo.ListAccounts(ctx, s.userID)
	if err != nil {
		return DashboardResponse{}, err
	}
	candidates, err := s.repo.PlannerAccounts(ctx, s.userID)
	if err != nil {
		return DashboardResponse{}, err
	}
	periods, err := s.repo.ListPeriods(ctx, s.userID)
	if err != nil {
		return DashboardResponse{}, err
	}
	stats, err := s.repo.AccountStats(ctx, s.userID)
	if err != nil {
		return DashboardResponse{}, err
	}
	counts, err := s.repo.StateCounts(ctx, s.userID)
	if err != nil {
		return DashboardResponse{}, err
	}
	current := map[int64]bool{}
	for _, p := range candidates {
		current[p.ID] = p.IsCurrent
	}

	out := DashboardResponse{Accounts: []AccountSummary{}, Phone: counts, Alerts: []Alert{}}
	for _, a := range accts {
		d := AccountSummary{ID: a.ID, Email: a.Email, Role: string(a.Role), Active: a.Active, Full: a.FullFlag, Current: current[a.ID],
			ActivePeriods: account.PeriodsOf(periods, a.ID), AccountStats: stats[a.ID]}
		if d.AccountStats == nil {
			d.AccountStats = &repository.AccountStats{Folders: []string{}}
		}
		if q, err := s.access.Quota(ctx, a); err != nil {
			d.StorageError = err.Error()
			out.Alerts = append(out.Alerts, Alert{ALERT_ACCOUNT_UNREACHABLE, a.Email + ": can't reach Google; reconnect the account if this persists"})
		} else {
			d.StorageFree = common.FormatGB(q.Remaining())
		}
		if a.FullFlag {
			out.Alerts = append(out.Alerts, Alert{ALERT_ACCOUNT_FULL, a.Email + " is full; new photos go to the next account"})
		}
		if d.PendingDelete > 0 {
			d.ToDeleteAlbum, _ = s.repo.AlbumLink(ctx, a.ID, repository.ToDeleteFolder)
			out.Alerts = append(out.Alerts, Alert{ALERT_TO_DELETE_WAITING, a.Email + ": photos are waiting in the 🗑 To delete album"})
		}
		out.Accounts = append(out.Accounts, d)
	}
	if _, _, why := planner.ChooseAccount(candidates, nil); why != "" {
		out.Alerts = append(out.Alerts, Alert{ALERT_NO_ACCOUNT_AVAILABLE, "No account can take new photos: connect another account or free up space"})
	}
	return out, nil
}

func (s *dashboardService) Where(ctx context.Context, q url.Values) (WhereResponse, error) {
	var from, to time.Time
	var err error
	switch {
	case q.Get("month") != "":
		from, err = time.Parse("2006-01", q.Get("month"))
		to = from.AddDate(0, 1, 0)
	case q.Get("from") != "" && q.Get("to") != "":
		from, err = time.Parse(time.DateOnly, q.Get("from"))
		if err == nil {
			to, err = time.Parse(time.DateOnly, q.Get("to"))
			to = to.AddDate(0, 0, 1) // inclusive end date
		}
	default:
		return WhereResponse{}, httperror.NewHTTPError(httperror.BAD_REQUEST, "give month=YYYY-MM or from=YYYY-MM-DD&to=YYYY-MM-DD")
	}
	if err != nil || !to.After(from) {
		return WhereResponse{}, httperror.NewHTTPError(httperror.BAD_REQUEST, "bad dates (format YYYY-MM or YYYY-MM-DD, from before to)")
	}
	res, err := s.repo.WhereTaken(ctx, s.userID, from, to)
	if err != nil {
		return WhereResponse{}, err
	}
	undated, err := s.repo.UndatedBackups(ctx, s.userID)
	if err != nil {
		return WhereResponse{}, err
	}
	return WhereResponse{From: from.Format(time.DateOnly), ToExclusive: to.Format(time.DateOnly), Accounts: res, UndatedBackups: undated}, nil
}
