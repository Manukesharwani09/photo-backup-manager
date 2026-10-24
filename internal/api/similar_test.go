package api

import (
	"net/http"
	"testing"

	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
)

func withPHash(f syncFile, h string) syncFile { f.PHash = &h; return f }

type similarResp struct {
	Groups []struct {
		Reason string
		Photos []repository.MediaDetail
	}
}

type toDeleteResp struct {
	Accounts []struct {
		AccountID int64  `json:"account_id"`
		AlbumURL  string `json:"album_url"`
		Added     int
		Waiting   int
	}
	NotInGoogle int `json:"not_in_google"`
}

type checkResp = CloudCheckResult

func TestSimilarToDeleteAndCloudCheck(t *testing.T) {
	e := setup(t)
	e.sync()
	orig := withPHash(file("WhatsApp/Images", "IMG-1.jpg", "original bytes"), "f0f0f0f0f0f0f0f0")
	resent := withPHash(file("WhatsApp/Images", "IMG-1-resent.jpg", "recompressed bytes"), "f0f0f0f0f0f0f0f3")
	other := withPHash(file("DCIM", "cat.jpg", "cat"), "0f0f0f0f0f0f0f0f")
	r := e.sync(orig, resent, other)
	// Look-alikes with different bytes are both uploaded (only identical bytes are skipped).
	if len(r.Plan) != 3 {
		t.Fatalf("all three must upload: %+v", r.Plan)
	}
	e.uploadPlan(r.Plan)

	var sim similarResp
	e.call("GET", "/similar", nil, http.StatusOK, &sim)
	if len(sim.Groups) != 1 || sim.Groups[0].Reason != "looks_alike" || len(sim.Groups[0].Photos) != 2 {
		t.Fatalf("similar: %+v", sim)
	}
	var dupID int64
	for _, p := range sim.Groups[0].Photos {
		if p.BackupStatus != "verified" || p.ProductURL == nil || len(p.OnDeviceFileIDs) != 1 {
			t.Fatalf("photo detail: %+v", p)
		}
		if p.Paths[0] == "WhatsApp/Images/IMG-1-resent.jpg" {
			dupID = p.MediaID
		}
	}

	// User keeps the original, removes the re-sent copy: phone copy via cleanup, cloud copy via the album.
	var td toDeleteResp
	e.call("POST", "/cloud/to-delete", map[string]any{"media_ids": []int64{dupID, dupID, 999999}}, http.StatusOK, &td)
	if len(td.Accounts) != 1 || td.Accounts[0].Added != 1 || td.Accounts[0].Waiting != 1 || td.Accounts[0].AlbumURL == "" || td.NotInGoogle != 1 {
		t.Fatalf("to-delete: %+v", td)
	}
	if len(e.g.added) != 1 {
		t.Fatalf("expected one To delete album with the item: %v", e.g.added)
	}
	for id, items := range e.g.added {
		if e.g.albums[id] != "rtA|"+ToDeleteAlbumTitle || len(items) != 1 {
			t.Fatalf("album %s = %q items %v", id, e.g.albums[id], items)
		}
	}
	// Marking again reuses the same album.
	e.call("POST", "/cloud/to-delete", map[string]any{"media_ids": []int64{dupID}}, http.StatusOK, &td)
	if len(e.g.added) != 1 {
		t.Fatal("a second To delete album was created")
	}

	// Before the user acts, the cloud check finds everything.
	var chk checkResp
	e.call("POST", "/cloud/check", nil, http.StatusOK, &chk)
	if chk.Checked != 3 || chk.StillThere != 3 || chk.DeletedInCloud != 0 {
		t.Fatalf("check 1: %+v", chk)
	}

	// The user deletes the "To delete" album's contents in Google Photos, and separately (by mistake)
	// deletes cat.jpg directly in Google Photos.
	e.g.mu.Lock()
	for id, desc := range e.g.items {
		if desc[len(desc)-len("IMG-1-resent.jpg"):] == "IMG-1-resent.jpg" || desc[len(desc)-len("cat.jpg"):] == "cat.jpg" {
			delete(e.g.items, id)
		}
	}
	e.g.mu.Unlock()
	e.call("POST", "/cloud/check", nil, http.StatusOK, &chk)
	if chk.DeletedInCloud != 2 || chk.StillThere != 1 || chk.Requeued != 1 {
		t.Fatalf("check 2: %+v", chk)
	}

	// Next sync: cat.jpg (deleted by mistake, still on the phone) is uploaded again;
	// the deliberately deleted duplicate is not.
	r = e.sync(orig, resent, other)
	if len(r.Plan) != 1 || r.Plan[0].DisplayName != "cat.jpg" {
		t.Fatalf("after check: plan %+v counts %v", r.Plan, r.Counts)
	}
	if r.Counts["excluded"] != 1 {
		t.Fatalf("deliberately deleted copy should be excluded: %v", r.Counts)
	}
	// A third check has nothing new to do.
	e.call("POST", "/cloud/check", nil, http.StatusOK, &chk)
	if chk.DeletedInCloud != 0 || chk.Checked != 1 {
		t.Fatalf("check 3: %+v", chk)
	}
}

func TestSimilarSameNameAndDate(t *testing.T) {
	e := setup(t)
	e.sync()
	when := "2025-12-31T23:59:00Z"
	a := file("DCIM/Camera", "IMG_9.jpg", "v1")
	b := file("Pictures/Edited", "IMG_9.jpg", "v2-resaved")
	for _, f := range []*syncFile{&a, &b} {
		f.DateTaken, f.Width, f.Height = &when, intPtr(4000), intPtr(3000)
	}
	e.uploadPlan(e.sync(a, b).Plan)
	var sim similarResp
	e.call("GET", "/similar", nil, http.StatusOK, &sim)
	if len(sim.Groups) != 1 || sim.Groups[0].Reason != "same_name_and_date" {
		t.Fatalf("got %+v", sim)
	}
}

func TestSimilarValidation(t *testing.T) {
	e := setup(t)
	e.sync()
	r := e.sync(withPHash(file("DCIM", "x.jpg", "x"), "not-hex-at-all!!"), withPHash(file("DCIM", "y.jpg", "y"), "abc"))
	if len(r.Rejected) != 2 {
		t.Fatalf("bad phash accepted: %+v", r)
	}
	e.call("POST", "/cloud/to-delete", map[string]any{"media_ids": []int64{}}, http.StatusBadRequest, nil)
	var sim similarResp
	e.call("GET", "/similar", nil, http.StatusOK, &sim)
	if len(sim.Groups) != 0 {
		t.Fatalf("no photos, no groups: %+v", sim)
	}
}

func TestCloudCheckSurvivesGoogleOutage(t *testing.T) {
	e := setup(t)
	e.sync()
	e.uploadPlan(e.sync(file("DCIM", "a.jpg", "A")).Plan)
	e.g.srv.Close() // Google unreachable
	var chk checkResp
	e.call("POST", "/cloud/check", nil, http.StatusOK, &chk)
	if chk.DeletedInCloud != 0 || len(chk.Errors) == 0 {
		t.Fatalf("an outage must not be treated as deletion: %+v", chk)
	}
	if n := e.scalar(`SELECT count(*) FROM backup_record WHERE status = 'verified'`); n != 1 {
		t.Fatalf("backup lost after outage: %d", n)
	}
}
