package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
)

type restoreResp struct {
	Items           []restoreItem
	AlreadyOnDevice int      `json:"already_on_device"`
	MissingInGoogle []string `json:"missing_in_google"`
	Tokens          map[int64]accessToken
}

func (e *env) restorePlan(device int64, folder string) restoreResp {
	e.t.Helper()
	var out restoreResp
	e.call("POST", "/restore/plan", map[string]any{"device_id": device, "folder": folder}, http.StatusOK, &out)
	return out
}

func hexSum(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestRestoreToNewPhone(t *testing.T) {
	e := setup(t)
	e.sync()
	lat, lng := 12.97, 77.59
	gps := file("DCIM/Camera", "gps.jpg", "GPS")
	gps.GPSLat, gps.GPSLng = &lat, &lng
	r := e.sync(gps, file("DCIM/Camera", "b.jpg", "B"), file("Pictures", "p.jpg", "P"))
	e.uploadPlan(r.Plan)

	// Old phone still has everything: nothing to restore there.
	if got := e.restorePlan(e.device, "DCIM"); len(got.Items) != 0 || got.AlreadyOnDevice != 2 {
		t.Fatalf("old phone: %+v", got)
	}

	// New phone (registered later) restores DCIM from Google.
	var dev struct {
		DeviceID int64 `json:"device_id"`
	}
	e.call("POST", "/devices", map[string]string{"model": "new"}, http.StatusCreated, &dev)
	got := e.restorePlan(dev.DeviceID, "DCIM")
	if len(got.Items) != 2 {
		t.Fatalf("new phone items: %+v", got)
	}
	var gpsItem *restoreItem
	for i := range got.Items {
		it := got.Items[i]
		if it.DownloadURL == "" || it.DownloadURL[len(it.DownloadURL)-2:] != "=d" || it.AccountID != e.primary {
			t.Fatalf("item %+v", it)
		}
		if it.DisplayName == "gps.jpg" {
			gpsItem = &got.Items[i]
		}
	}
	if gpsItem == nil || gpsItem.GPSLat == nil || *gpsItem.GPSLat != lat || gpsItem.RelativePath != "DCIM/Camera" {
		t.Fatalf("GPS and path must come back for write-back: %+v", gpsItem)
	}
	if got.Tokens[e.primary].AccessToken == "" {
		t.Fatal("no download token")
	}

	// Restored bytes differ from the original (Google rewrote metadata): linked, not re-uploaded.
	var done struct {
		Linked   int
		Rejected []string
	}
	items := []map[string]any{}
	for _, it := range got.Items {
		items = append(items, map[string]any{"media_id": it.MediaID, "relative_path": it.RelativePath,
			"display_name": it.DisplayName, "size": 10, "sha256": hexSum("restored-" + it.DisplayName)})
	}
	items = append(items, map[string]any{"media_id": 999999, "relative_path": "X", "display_name": "x.jpg", "sha256": hexSum("x")})
	items = append(items, map[string]any{"media_id": got.Items[0].MediaID, "relative_path": "X", "display_name": "bad.jpg", "sha256": "zz"})
	e.call("POST", "/restore/done", map[string]any{"device_id": dev.DeviceID, "items": items}, http.StatusOK, &done)
	if done.Linked != 2 || len(done.Rejected) != 2 {
		t.Fatalf("restore done: %+v", done)
	}
	// The new phone's next sync (its first: start point) and a later one must not upload the restored files.
	newPhone := []syncFile{
		{RelativePath: "DCIM/Camera", DisplayName: "gps.jpg", Size: 10, SHA256: hexSum("restored-gps.jpg"), Source: "camera"},
		{RelativePath: "DCIM/Camera", DisplayName: "b.jpg", Size: 10, SHA256: hexSum("restored-b.jpg"), Source: "camera"},
	}
	for i := 0; i < 2; i++ {
		var out syncResponse
		e.call("POST", "/sync", map[string]any{"device_id": dev.DeviceID, "files": newPhone}, http.StatusOK, &out)
		if len(out.Plan) != 0 || out.Counts["backed_up"] != 2 {
			t.Fatalf("sync %d after restore: %+v", i, out)
		}
	}
	// Restored copies point at the same Google items: the dashboard must not count them twice.
	var d dashResp
	e.call("GET", "/dashboard", nil, http.StatusOK, &d)
	if d.Accounts[0].Items != 3 {
		t.Fatalf("primary holds 3 photos, dashboard says %d", d.Accounts[0].Items)
	}
	var where struct{ Accounts []struct{ Items int } }
	e.call("GET", "/photos/where?from=1970-01-01&to=2100-01-01", nil, http.StatusOK, &where)
	if len(where.Accounts) != 0 {
		t.Fatalf("test photos are undated, expected no dated matches: %+v", where)
	}
	// Restoring again: everything is already there.
	if again := e.restorePlan(dev.DeviceID, "DCIM"); len(again.Items) != 0 || again.AlreadyOnDevice != 2 {
		t.Fatalf("second restore: %+v", again)
	}
}

func TestRestoreReportsItemsDeletedInGoogle(t *testing.T) {
	e := setup(t)
	e.sync()
	r := e.sync(file("DCIM", "a.jpg", "A"))
	e.uploadPlan(r.Plan)
	var cleaned map[string]int64
	e.call("POST", "/cleanup/done", map[string]any{"device_id": e.device, "file_ids": []int64{r.Plan[0].FileID}}, http.StatusOK, &cleaned)
	e.g.mu.Lock()
	e.g.items = map[string]string{} // the user deleted it in Google Photos
	e.g.mu.Unlock()
	got := e.restorePlan(e.device, "DCIM")
	if len(got.Items) != 0 || len(got.MissingInGoogle) != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestRestoreValidation(t *testing.T) {
	e := setup(t)
	e.call("POST", "/restore/plan", map[string]any{"device_id": e.device, "folder": "/"}, http.StatusBadRequest, nil)
	e.call("POST", "/restore/plan", map[string]any{"device_id": 4242, "folder": "DCIM"}, http.StatusNotFound, nil)
	e.call("POST", "/restore/done", map[string]any{"device_id": 4242, "items": []any{}}, http.StatusNotFound, nil)
}
