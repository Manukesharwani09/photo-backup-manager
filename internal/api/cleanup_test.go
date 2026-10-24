package api

import (
	"net/http"
	"testing"
)

type cleanupResp struct {
	Eligible   []cleanupFile
	TotalBytes int64 `json:"total_bytes"`
	Kept       map[string]int
}

func (e *env) cleanupPlan(folders ...string) cleanupResp {
	e.t.Helper()
	var out cleanupResp
	e.call("POST", "/cleanup/plan", map[string]any{"device_id": e.device, "folders": folders}, http.StatusOK, &out)
	return out
}

func names(fs []cleanupFile) map[string]bool {
	m := map[string]bool{}
	for _, f := range fs {
		m[f.RelativePath+"/"+f.DisplayName] = true
	}
	return m
}

func TestCleanupPlanOnlyVerifiedUnprotected(t *testing.T) {
	e := setup(t)
	startPoint := file("DCIM/Camera", "old.jpg", "OLD")
	e.sync(startPoint)
	e.call("PUT", "/rules", []ruleJSON{
		{PathGlob: "DCIM/Camera/Docs", Cleanup: "protected"},
		{PathGlob: "DCIM/Private", Backup: "exclude"},
	}, http.StatusOK, nil)

	up := file("DCIM/Camera", "up.jpg", "UP")              // will be verified
	protectedF := file("DCIM/Camera/Docs", "id.jpg", "ID") // verified but protected
	pendingF := file("DCIM/Camera", "pending.jpg", "PEN")  // never uploaded
	excludedF := file("DCIM/Private", "x.jpg", "X")        // excluded
	other := file("Pictures", "p.jpg", "P")                // verified, outside the chosen folder
	r := e.sync(startPoint, up, protectedF, pendingF, excludedF, other)
	var toUpload []planItem
	for _, p := range r.Plan {
		if p.DisplayName != "pending.jpg" {
			toUpload = append(toUpload, p)
		}
	}
	e.uploadPlan(toUpload)

	got := e.cleanupPlan("DCIM") // DCIM and every sub-folder
	if n := names(got.Eligible); len(n) != 1 || !n["DCIM/Camera/up.jpg"] {
		t.Fatalf("eligible: %v", n)
	}
	if got.TotalBytes != int64(len("UP")) || got.Eligible[0].AccountID != e.primary || got.Eligible[0].ProductURL == "" {
		t.Fatalf("details: %+v", got)
	}
	// start point + pending + excluded are not backed up; Docs is protected.
	if got.Kept["not_backed_up"] != 3 || got.Kept["protected"] != 1 {
		t.Fatalf("kept: %v", got.Kept)
	}

	// The phone deletes only what the user confirmed; a bogus id and an unbacked file are ignored.
	var done map[string]int64
	upID := got.Eligible[0].FileID
	var pendingID int64
	for _, p := range r.Plan {
		if p.DisplayName == "pending.jpg" {
			pendingID = p.FileID
		}
	}
	e.call("POST", "/cleanup/done", map[string]any{"device_id": e.device, "file_ids": []int64{upID, pendingID, 999999}}, http.StatusOK, &done)
	if done["marked_removed"] != 1 || done["ignored"] != 2 {
		t.Fatalf("done: %v", done)
	}
	if got := e.cleanupPlan("DCIM"); len(got.Eligible) != 0 {
		t.Fatalf("already removed file offered again: %+v", got.Eligible)
	}
	// Choosing the protected folder itself yields nothing to delete.
	if got := e.cleanupPlan("DCIM/Camera/Docs"); len(got.Eligible) != 0 || got.Kept["protected"] != 1 {
		t.Fatalf("protected folder: %+v", got)
	}
}

func TestCleanupUnverifiedIsKept(t *testing.T) {
	e := setup(t)
	e.sync()
	r := e.sync(file("DCIM", "a.jpg", "A"))
	e.uploadPlan(r.Plan)
	if _, err := e.st.Pool.Exec(t.Context(), `UPDATE backup_record SET status = 'uploaded'`); err != nil {
		t.Fatal(err)
	}
	got := e.cleanupPlan("DCIM")
	if len(got.Eligible) != 0 || got.Kept["not_verified"] != 1 {
		t.Fatalf("unverified backup must not be cleaned: %+v", got)
	}
	var done map[string]int64
	e.call("POST", "/cleanup/done", map[string]any{"device_id": e.device, "file_ids": []int64{r.Plan[0].FileID}}, http.StatusOK, &done)
	if done["marked_removed"] != 0 {
		t.Fatalf("unverified file marked removed: %v", done)
	}
}

func TestCleanupValidation(t *testing.T) {
	e := setup(t)
	e.call("POST", "/cleanup/plan", map[string]any{"device_id": e.device, "folders": []string{}}, http.StatusBadRequest, nil)
	e.call("POST", "/cleanup/plan", map[string]any{"device_id": 4242, "folders": []string{"DCIM"}}, http.StatusNotFound, nil)
	e.call("POST", "/cleanup/done", map[string]any{"device_id": 4242, "file_ids": []int64{1}}, http.StatusNotFound, nil)
}
