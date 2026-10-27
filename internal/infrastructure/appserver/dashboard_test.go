package appserver

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Manukesharwani09/photo-backup-manager/internal/application/backup"
)

type dashResp struct {
	Accounts []struct {
		ID            int64
		Email         string
		Current       bool
		Full          bool
		StorageFree   string           `json:"storage_free"`
		ActivePeriods []map[string]any `json:"active_periods"`
		Items         int
		TakenFrom     *string `json:"photos_taken_from"`
		TakenTo       *string `json:"photos_taken_to"`
		Folders       []string
		PendingDelete int    `json:"waiting_in_to_delete"`
		ToDeleteAlbum string `json:"to_delete_album_url"`
	}
	Phone  map[string]int `json:"phone_files"`
	Alerts []struct{ Kind, Message string }
}

func dated(folder, name, content, when string) backup.SyncFile {
	f := file(folder, name, content)
	f.DateTaken = &when
	return f
}

func alertKinds(d dashResp) map[string]int {
	m := map[string]int{}
	for _, a := range d.Alerts {
		m[a.Kind]++
	}
	return m
}

func TestDashboardAndPhotosWhere(t *testing.T) {
	e := setup(t)
	e.sync(file("DCIM", "old.jpg", "OLD")) // start point

	// March 2024 photos go to primary; then primary is full and June 2024 photos go to overflow.
	r := e.sync(file("DCIM", "old.jpg", "OLD"),
		dated("DCIM/Camera", "m1.jpg", "M1", "2024-03-05T10:00:00Z"),
		dated("WhatsApp/Images", "m2.jpg", "M2", "2024-03-20T10:00:00Z"))
	e.uploadPlan(r.Plan)
	e.call("POST", fmt.Sprintf("/v1/accounts/%d/full", e.primary), nil, http.StatusOK, nil)
	r = e.sync(dated("DCIM/Camera", "j1.jpg", "J1", "2024-06-01T10:00:00Z"),
		dated("DCIM/Camera", "late.jpg", "L", "2024-03-31T23:00:00Z"), // an old photo received late
		file("DCIM/Camera", "nodate.jpg", "ND"))
	e.uploadPlan(r.Plan)

	var d dashResp
	e.call("GET", "/v1/dashboard", nil, http.StatusOK, &d)
	if len(d.Accounts) != 2 {
		t.Fatalf("accounts: %+v", d.Accounts)
	}
	p, o := d.Accounts[0], d.Accounts[1]
	if p.Current || !p.Full || p.Items != 2 || len(p.Folders) != 2 || p.StorageFree != "14 GB" {
		t.Fatalf("primary: %+v", p)
	}
	if p.TakenFrom == nil || (*p.TakenFrom)[:10] != "2024-03-05" || (*p.TakenTo)[:10] != "2024-03-20" {
		t.Fatalf("primary date range: %v %v", p.TakenFrom, p.TakenTo)
	}
	if len(p.ActivePeriods) != 1 || p.ActivePeriods[0]["end_reason"] != "full" {
		t.Fatalf("primary periods: %+v", p.ActivePeriods)
	}
	if !o.Current || o.Items != 3 || len(o.ActivePeriods) != 1 || o.ActivePeriods[0]["ended_at"] != nil {
		t.Fatalf("overflow: %+v", o)
	}
	if d.Phone["existing_skip"] != 1 || d.Phone["backed_up"] != 5 || d.Phone["pending"] != 0 {
		t.Fatalf("phone counts: %v", d.Phone)
	}
	if k := alertKinds(d); k["account_full"] != 1 || k["no_account_available"] != 0 {
		t.Fatalf("alerts: %+v", d.Alerts)
	}

	// "Where are my March 2024 photos?" -> both accounts (one arrived late, after the switch).
	var where struct {
		Accounts []struct {
			AccountID int64 `json:"account_id"`
			Items     int
			Folders   []string
		}
		Undated int `json:"undated_backups"`
	}
	e.call("GET", "/v1/photos/where?month=2024-03", nil, http.StatusOK, &where)
	if len(where.Accounts) != 2 || where.Accounts[0].Items != 2 || where.Accounts[1].Items != 1 || where.Undated != 1 {
		t.Fatalf("march: %+v", where)
	}
	e.call("GET", "/v1/photos/where?from=2024-06-01&to=2024-06-01", nil, http.StatusOK, &where) // inclusive end
	if len(where.Accounts) != 1 || where.Accounts[0].AccountID != e.over || where.Accounts[0].Items != 1 {
		t.Fatalf("june 1: %+v", where)
	}
	e.call("GET", "/v1/photos/where?month=1999-01", nil, http.StatusOK, &where)
	if len(where.Accounts) != 0 {
		t.Fatalf("empty period: %+v", where)
	}
	for _, bad := range []string{"", "?month=March", "?from=2024-06-02&to=2024-06-01", "?from=2024-01-01"} {
		e.call("GET", "/v1/photos/where"+bad, nil, http.StatusBadRequest, nil)
	}

	// Both full -> "no account available" alert; a To delete request shows its album.
	e.call("POST", fmt.Sprintf("/v1/accounts/%d/full", e.over), nil, http.StatusOK, nil)
	var mids []int64
	e.st.Pool.QueryRow(t.Context(), `SELECT array_agg(media_id) FROM backup_record`).Scan(&mids)
	e.call("POST", "/v1/cloud/to-delete", map[string]any{"media_ids": mids[:1]}, http.StatusOK, nil)
	e.call("GET", "/v1/dashboard", nil, http.StatusOK, &d)
	k := alertKinds(d)
	if k["no_account_available"] != 1 || k["account_full"] != 2 || k["to_delete_waiting"] != 1 {
		t.Fatalf("alerts after: %+v", d.Alerts)
	}
	waiting := 0
	for _, a := range d.Accounts {
		if a.PendingDelete > 0 && a.ToDeleteAlbum != "" {
			waiting++
		}
	}
	if waiting != 1 {
		t.Fatalf("to-delete album link missing: %+v", d.Accounts)
	}
}

func TestDashboardEmptyAndUnreachable(t *testing.T) {
	e := setup(t)
	var d dashResp
	e.call("GET", "/v1/dashboard", nil, http.StatusOK, &d)
	if d.Accounts[0].Items != 0 || d.Accounts[0].Folders == nil {
		t.Fatalf("empty account stats: %+v", d.Accounts[0])
	}
	e.g.srv.Close()
	e.call("GET", "/v1/dashboard", nil, http.StatusOK, &d)
	if alertKinds(d)["account_unreachable"] != 2 {
		t.Fatalf("Google down should alert, not fail: %+v", d.Alerts)
	}
}
