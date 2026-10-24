package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
)

// CloudCheckResult summarises one pass over all live backups.
type CloudCheckResult struct {
	Checked        int      `json:"checked"`
	StillThere     int      `json:"still_there"`
	DeletedInCloud int      `json:"deleted_in_cloud"`
	Requeued       int64    `json:"requeued_for_upload"` // phone copies that will be uploaded again
	Errors         []string `json:"errors"`
}

// CheckCloud asks Google, 50 items per request, which backed-up items still exist and records the
// ones the user deleted in Google Photos. Quota cost: about one request per 50 backed-up photos.
func (s *Server) CheckCloud(ctx context.Context) (CloudCheckResult, error) {
	res := CloudCheckResult{Errors: []string{}}
	byAccount, order, err := s.Store.LiveRecordsByAccount(ctx, s.UserID)
	if err != nil {
		return res, err
	}
	for _, acctID := range order {
		acct, err := s.Store.AccountOwned(ctx, s.UserID, acctID)
		if err != nil {
			return res, err
		}
		client, err := s.Client(ctx, acct)
		if err != nil {
			res.Errors = append(res.Errors, acct.Email+": "+err.Error())
			continue
		}
		recs := byAccount[acctID]
		for start := 0; start < len(recs); start += google.MaxBatch {
			chunk := recs[start:min(start+google.MaxBatch, len(recs))]
			ids := make([]string, len(chunk))
			for i, r := range chunk {
				ids[i] = r.MediaItemID
			}
			found, err := google.BatchGet(ctx, client, uniq(ids))
			if err != nil {
				// Don't treat an API failure as "deleted": skip this chunk and try next time.
				res.Errors = append(res.Errors, acct.Email+": "+err.Error())
				continue
			}
			var present []int64
			for _, r := range chunk {
				res.Checked++
				if _, ok := found[r.MediaItemID]; ok {
					present = append(present, r.ID)
					continue
				}
				n, err := s.Store.MarkDeletedInCloud(ctx, r)
				if err != nil {
					return res, err
				}
				res.DeletedInCloud++
				res.Requeued += n
			}
			res.StillThere += len(present)
			if err := s.Store.TouchChecked(ctx, present); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

func (s *Server) cloudCheck(w http.ResponseWriter, r *http.Request) {
	res, err := s.CheckCloud(r.Context())
	if err != nil {
		httpError(w, "cloud check", err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// RunCloudChecks repeats CheckCloud every interval until ctx ends. interval <= 0 disables it.
func (s *Server) RunCloudChecks(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			res, err := s.CheckCloud(ctx)
			if err != nil {
				log.Printf("cloud check: %v", err)
				continue
			}
			log.Printf("cloud check: %+v", res)
		}
	}
}
