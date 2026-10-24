package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"golang.org/x/oauth2"

	"github.com/Manukesharwani09/photo-backup-manager/internal/crypto"
	"github.com/Manukesharwani09/photo-backup-manager/internal/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/store"
)

// fakeGoogle imitates the OAuth token endpoint and the Photos Library API.
// Access tokens are "at:<refresh token>" so the Photos side knows which account is calling.
type fakeGoogle struct {
	mu         sync.Mutex
	srv        *httptest.Server
	albums     map[string]string   // album id -> "account|title"
	items      map[string]string   // media item id -> "account|album|description"
	fullFor    map[string]bool     // account refresh token -> report storage full
	added      map[string][]string // album id -> media item ids added with batchAddMediaItems
	nextID     int
	batchCalls int
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	f := &fakeGoogle{albums: map[string]string{}, items: map[string]string{}, fullFor: map[string]bool{}, added: map[string][]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"access_token": "at:" + r.Form.Get("refresh_token"), "token_type": "Bearer", "expires_in": 3600})
	})
	mux.HandleFunc("POST /albums", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Album struct{ Title string } }
		json.NewDecoder(r.Body).Decode(&in)
		f.mu.Lock()
		f.nextID++
		id := fmt.Sprintf("album%d", f.nextID)
		f.albums[id] = account(r) + "|" + in.Album.Title
		f.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]string{"id": id, "productUrl": "https://photos.google.com/album/" + id})
	})
	mux.HandleFunc("POST /mediaItems:batchCreate", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			AlbumID       string
			NewMediaItems []struct {
				Description     string
				SimpleMediaItem struct{ UploadToken string }
			}
		}
		json.NewDecoder(r.Body).Decode(&in)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.batchCalls++
		acct := account(r)
		var res []map[string]any
		for _, it := range in.NewMediaItems {
			tok := it.SimpleMediaItem.UploadToken
			if f.fullFor[acct] {
				res = append(res, map[string]any{"uploadToken": tok, "status": map[string]any{"code": 8, "message": "Not enough storage"}})
				continue
			}
			if strings.HasPrefix(tok, "bad") {
				res = append(res, map[string]any{"uploadToken": tok, "status": map[string]any{"code": 3, "message": "Failed: invalid token"}})
				continue
			}
			f.nextID++
			id := fmt.Sprintf("item%d", f.nextID)
			f.items[id] = acct + "|" + in.AlbumID + "|" + it.Description
			res = append(res, map[string]any{"uploadToken": tok, "status": map[string]any{"message": "Success"},
				"mediaItem": map[string]string{"id": id, "productUrl": "https://photos.google.com/lr/" + id}})
		}
		json.NewEncoder(w).Encode(map[string]any{"newMediaItemResults": res})
	})
	mux.HandleFunc("POST /albums/{id}", func(w http.ResponseWriter, r *http.Request) { // {id} = "<album>:batchAddMediaItems"
		albumID, action, _ := strings.Cut(r.PathValue("id"), ":")
		var in struct{ MediaItemIDs []string }
		json.NewDecoder(r.Body).Decode(&in)
		f.mu.Lock()
		defer f.mu.Unlock()
		if action != "batchAddMediaItems" || f.albums[albumID] == "" {
			http.Error(w, "bad album call", http.StatusBadRequest)
			return
		}
		for _, id := range in.MediaItemIDs {
			if f.items[id] == "" {
				http.Error(w, "unknown media item", http.StatusBadRequest)
				return
			}
			f.added[albumID] = append(f.added[albumID], id)
		}
		io.WriteString(w, "{}")
	})
	mux.HandleFunc("GET /about", func(w http.ResponseWriter, r *http.Request) { // Drive storage quota
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"storageQuota":{"limit":"16106127360","usage":"1073741824"}}`)
	})
	mux.HandleFunc("GET /mediaItems:batchGet", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		var res []map[string]any
		for _, id := range r.URL.Query()["mediaItemIds"] {
			if _, ok := f.items[id]; ok {
				res = append(res, map[string]any{"mediaItem": map[string]string{"id": id,
					"baseUrl": "https://lh3.fake/" + id, "mimeType": "image/jpeg", "productUrl": "https://photos.google.com/lr/" + id}})
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"mediaItemResults": res})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	oldPhotos, oldDrive := google.PhotosAPI, google.DriveAPI
	google.PhotosAPI, google.DriveAPI = f.srv.URL, f.srv.URL
	t.Cleanup(func() { google.PhotosAPI, google.DriveAPI = oldPhotos, oldDrive })
	return f
}

func account(r *http.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer at:")
}

type env struct {
	t       *testing.T
	h       http.Handler
	st      *store.Store
	g       *fakeGoogle
	primary int64
	over    int64
	device  int64
}

func setup(t *testing.T) *env {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	st, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Pool.Close)
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `TRUNCATE app_user, device, account, account_period, media, device_file,
		album, backup_record, folder_rule RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	g := newFakeGoogle(t)
	key := make([]byte, 32)
	uid, _ := st.DefaultUserID(ctx)
	enc := func(s string) []byte { b, _ := crypto.Seal(key, []byte(s)); return b }
	p, err := st.UpsertAccount(ctx, uid, "primary@gmail.com", "", "primary", enc("rtA"))
	if err != nil {
		t.Fatal(err)
	}
	o, _ := st.UpsertAccount(ctx, uid, "overflow@gmail.com", "", "overflow", enc("rtB"))

	s := &Server{Store: st, TokenKey: key, UserID: uid, OAuth: &oauth2.Config{ClientID: "c", ClientSecret: "s",
		Endpoint: oauth2.Endpoint{TokenURL: g.srv.URL + "/token", AuthStyle: oauth2.AuthStyleInParams}}}
	e := &env{t: t, h: s.Routes(), st: st, g: g, primary: p, over: o}
	var dev struct {
		DeviceID int64 `json:"device_id"`
	}
	e.call("POST", "/devices", map[string]string{"model": "test"}, http.StatusCreated, &dev)
	e.device = dev.DeviceID
	return e
}

func (e *env) call(method, path string, body any, wantStatus int, out any) string {
	e.t.Helper()
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(b)))
	if rec.Code != wantStatus {
		e.t.Fatalf("%s %s: status %d, want %d: %s", method, path, rec.Code, wantStatus, rec.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			e.t.Fatalf("decode %s: %v: %s", path, err, rec.Body.String())
		}
	}
	return rec.Body.String()
}

func file(folder, name, content string) syncFile {
	sum := sha256.Sum256([]byte(content))
	return syncFile{RelativePath: folder, DisplayName: name, Size: int64(len(content)), SHA256: hex.EncodeToString(sum[:]), Source: "camera"}
}

func (e *env) sync(files ...syncFile) syncResponse {
	e.t.Helper()
	var out syncResponse
	e.call("POST", "/sync", map[string]any{"device_id": e.device, "files": files}, http.StatusOK, &out)
	return out
}

func (e *env) upload(items ...uploadItem) map[int64]uploadResult {
	e.t.Helper()
	var out struct{ Results []uploadResult }
	e.call("POST", "/uploads", map[string]any{"device_id": e.device, "items": items}, http.StatusOK, &out)
	m := map[int64]uploadResult{}
	for _, r := range out.Results {
		m[r.FileID] = r
	}
	return m
}

// uploadPlan simulates the phone uploading every planned file and reporting tokens.
func (e *env) uploadPlan(p []planItem) map[int64]uploadResult {
	var items []uploadItem
	for _, it := range p {
		items = append(items, uploadItem{FileID: it.FileID, AccountID: it.AccountID, UploadToken: fmt.Sprintf("tok-%d", it.FileID)})
	}
	return e.upload(items...)
}

func (e *env) scalar(q string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.st.Pool.QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func planFor(p []planItem, name string) *planItem {
	for i := range p {
		if p[i].DisplayName == name {
			return &p[i]
		}
	}
	return nil
}

func TestFullBackupFlow(t *testing.T) {
	e := setup(t)
	old1, old2 := file("DCIM/Camera", "old1.jpg", "o1"), file("WhatsApp/Images", "old2.jpg", "o2")

	// 1. First scan only records the start point: nothing is uploaded.
	r := e.sync(old1, old2)
	if !r.FirstScan || len(r.Plan) != 0 || r.Counts["existing_skip"] != 2 {
		t.Fatalf("first scan: %+v", r)
	}

	// Rules: Documents -> primary, protected; Private excluded.
	e.call("PUT", "/rules", []ruleJSON{
		{PathGlob: "Documents", TargetAccountID: &e.primary, Cleanup: "protected"},
		{PathGlob: "Private", Backup: "exclude"},
	}, http.StatusOK, nil)

	newC := file("DCIM/Camera/", "c.jpg", "C")
	docD := file("Documents/Bills", "d.jpg", "D")
	privE := file("Private", "e.jpg", "E")
	dupF := file("Pictures/Saved", "copy-of-c.jpg", "C") // same bytes as c.jpg
	r = e.sync(old1, old2, newC, docD, privE, dupF)
	if r.FirstScan {
		t.Fatal("second sync reported first scan")
	}
	if r.Counts["existing_skip"] != 2 || r.Counts["excluded"] != 1 || len(r.Plan) != 2 {
		t.Fatalf("second sync: counts %v plan %+v", r.Counts, r.Plan)
	}
	pc, pd := planFor(r.Plan, "c.jpg"), planFor(r.Plan, "d.jpg")
	if pc == nil || pd == nil || planFor(r.Plan, "copy-of-c.jpg") != nil {
		t.Fatalf("plan should hold c and d once each: %+v", r.Plan)
	}
	if pc.AccountID != e.primary || pd.AccountID != e.primary || pc.AlbumTitle != "DCIM › Camera" || pd.AlbumTitle != "Documents › Bills" {
		t.Fatalf("plan details: %+v %+v", pc, pd)
	}
	if tok := r.Tokens[e.primary]; tok.AccessToken != "at:rtA" || len(r.Tokens) != 1 {
		t.Fatalf("tokens: %+v", r.Tokens)
	}

	// 2. Phone uploads; backend creates items in per-folder albums and verifies them.
	res := e.uploadPlan(r.Plan)
	for _, p := range r.Plan {
		if res[p.FileID].Status != "backed_up" || !res[p.FileID].Verified || res[p.FileID].ProductURL == "" {
			t.Fatalf("upload %s: %+v", p.DisplayName, res[p.FileID])
		}
	}
	if len(e.g.albums) != 2 {
		t.Fatalf("albums: %v", e.g.albums)
	}
	for _, desc := range e.g.items {
		if !strings.Contains(desc, "DCIM/Camera/c.jpg") && !strings.Contains(desc, "Documents/Bills/d.jpg") {
			t.Fatalf("description should be the original path: %q", desc)
		}
	}
	if n := e.scalar(`SELECT count(*) FROM backup_record WHERE status = 'verified'`); n != 2 {
		t.Fatalf("verified records: %d", n)
	}

	// 3. Re-sync: nothing to upload; the duplicate copy now counts as backed up.
	r = e.sync(old1, old2, newC, docD, privE, dupF)
	if len(r.Plan) != 0 || r.Counts["backed_up"] != 3 {
		t.Fatalf("re-sync: counts %v plan %+v", r.Counts, r.Plan)
	}
	// Uploading an already-backed-up file again is refused.
	again := e.upload(uploadItem{FileID: pc.FileID, AccountID: e.primary, UploadToken: "tok-again"})
	if again[pc.FileID].Status != "already_backed_up" {
		t.Fatalf("re-upload: %+v", again[pc.FileID])
	}

	// 4. Primary is full: new camera photos go to overflow (period switches), but Documents wait.
	e.call("POST", fmt.Sprintf("/accounts/%d/full", e.primary), nil, http.StatusOK, nil)
	newG := file("DCIM/Camera", "g.jpg", "G")
	docH := file("Documents/Bills", "h.jpg", "H")
	r = e.sync(newG, docH)
	pg := planFor(r.Plan, "g.jpg")
	if pg == nil || pg.AccountID != e.over || len(r.Plan) != 1 {
		t.Fatalf("after full: plan %+v", r.Plan)
	}
	if len(r.Waiting) != 1 || r.Waiting[0].Reason != "target_account_full" {
		t.Fatalf("documents should wait for the full primary: %+v", r.Waiting)
	}
	if e.scalar(`SELECT count(*) FROM account_period WHERE account_id = $1 AND ended_at IS NOT NULL AND end_reason = 'full'`, e.primary) != 1 ||
		e.scalar(`SELECT count(*) FROM account_period WHERE account_id = $1 AND ended_at IS NULL`, e.over) != 1 {
		t.Fatal("active period did not move from primary to overflow")
	}
	res = e.uploadPlan(r.Plan)
	if res[pg.FileID].Status != "backed_up" {
		t.Fatalf("overflow upload: %+v", res[pg.FileID])
	}
	// Overflow gets its own album for the same folder.
	if e.scalar(`SELECT count(*) FROM album WHERE folder_path = 'DCIM/Camera'`) != 2 {
		t.Fatal("expected a DCIM/Camera album in each account")
	}

	// 5. Every account full: new non-rule photos wait with a reason.
	e.call("POST", fmt.Sprintf("/accounts/%d/full", e.over), nil, http.StatusOK, nil)
	r = e.sync(file("DCIM/Camera", "i.jpg", "I"))
	if len(r.Plan) != 0 || len(r.Waiting) != 1 || r.Waiting[0].Reason != "no_account_available" {
		t.Fatalf("all full: %+v", r)
	}
}

func TestStorageFullDuringUpload(t *testing.T) {
	e := setup(t)
	e.sync() // first scan with an empty phone
	e.g.fullFor["rtA"] = true
	r := e.sync(file("DCIM/Camera", "a.jpg", "A"), file("DCIM/Camera", "b.jpg", "B"))
	res := e.uploadPlan(r.Plan)
	for _, p := range r.Plan {
		if res[p.FileID].Status != "account_full" {
			t.Fatalf("got %+v", res[p.FileID])
		}
	}
	if e.scalar(`SELECT count(*) FROM account WHERE id = $1 AND full_flag`, e.primary) != 1 {
		t.Fatal("primary not marked full")
	}
	// Files stay pending; the next sync routes them to the overflow account.
	r = e.sync(file("DCIM/Camera", "a.jpg", "A"), file("DCIM/Camera", "b.jpg", "B"))
	if len(r.Plan) != 2 || r.Plan[0].AccountID != e.over {
		t.Fatalf("retry plan: %+v", r.Plan)
	}
	res = e.uploadPlan(r.Plan)
	if res[r.Plan[0].FileID].Status != "backed_up" {
		t.Fatalf("retry upload: %+v", res[r.Plan[0].FileID])
	}
}

func TestUploadEdgeCases(t *testing.T) {
	e := setup(t)
	e.sync()
	r := e.sync(file("DCIM", "a.jpg", "A"), file("DCIM", "b.jpg", "B"))
	a, b := planFor(r.Plan, "a.jpg"), planFor(r.Plan, "b.jpg")
	res := e.upload(
		uploadItem{FileID: a.FileID, AccountID: e.primary, UploadToken: "bad-token"}, // Google rejects it
		uploadItem{FileID: b.FileID, AccountID: e.primary, UploadToken: ""},          // empty token
		uploadItem{FileID: 999999, AccountID: e.primary, UploadToken: "x"},           // not this device's file
		uploadItem{FileID: a.FileID, AccountID: e.primary, UploadToken: "dup"},       // duplicate entry ignored
	)
	if res[a.FileID].Status != "failed" || res[b.FileID].Status != "invalid" || res[999999].Status != "invalid" {
		t.Fatalf("got %+v", res)
	}
	// Failed and invalid files remain pending and are planned again.
	r = e.sync(file("DCIM", "a.jpg", "A"), file("DCIM", "b.jpg", "B"))
	if len(r.Plan) != 2 {
		t.Fatalf("expected both planned again: %+v", r.Plan)
	}
	// Unknown account id in an upload.
	res = e.upload(uploadItem{FileID: a.FileID, AccountID: 424242, UploadToken: "t"})
	if res[a.FileID].Status != "invalid" {
		t.Fatalf("unknown account: %+v", res[a.FileID])
	}
	if e.g.batchCalls != 1 {
		t.Fatalf("only the bad-token item should have reached Google, batchCreate calls = %d", e.g.batchCalls)
	}
}

func TestSyncValidationAndChangedContent(t *testing.T) {
	e := setup(t)
	// Unknown device.
	e.call("POST", "/sync", map[string]any{"device_id": 777, "files": []syncFile{}}, http.StatusNotFound, nil)
	// Unknown JSON field is rejected (catches app/backend contract drift).
	e.call("POST", "/sync", map[string]any{"device_id": e.device, "bogus": 1}, http.StatusBadRequest, nil)

	old := file("DCIM", "x.jpg", "v1")
	e.sync(old) // start point
	bad := []syncFile{
		{RelativePath: "DCIM", DisplayName: "short.jpg", SHA256: "abc"},
		{RelativePath: "DCIM", DisplayName: "", SHA256: old.SHA256},
		{RelativePath: "DCIM", DisplayName: "src.jpg", SHA256: old.SHA256, Source: "telepathy"},
		{RelativePath: "DCIM", DisplayName: "date.jpg", SHA256: old.SHA256, DateTaken: strPtr("yesterday")},
		{RelativePath: "DCIM", DisplayName: "gps.jpg", SHA256: old.SHA256, GPSLat: f64(95), GPSLng: f64(10)},
		{RelativePath: "DCIM", DisplayName: "halfgps.jpg", SHA256: old.SHA256, GPSLat: f64(10)},
	}
	r := e.sync(bad...)
	if len(r.Rejected) != len(bad) || len(r.Plan) != 0 {
		t.Fatalf("rejected %v plan %+v", r.Rejected, r.Plan)
	}
	// Same path, unchanged: still skipped. Same path, new content (edited after the start point): uploaded.
	if r := e.sync(old); len(r.Plan) != 0 || r.Counts["existing_skip"] != 1 {
		t.Fatalf("unchanged: %+v", r)
	}
	if r := e.sync(file("DCIM", "x.jpg", "v2")); len(r.Plan) != 1 {
		t.Fatalf("edited file should be planned: %+v", r)
	}
	// Valid optional metadata is accepted.
	ok := file("DCIM", "meta.jpg", "M")
	ok.DateTaken, ok.GPSLat, ok.GPSLng, ok.Width, ok.Height = strPtr("2024-03-01T10:00:00Z"), f64(12.97), f64(77.59), intPtr(4000), intPtr(3000)
	if r := e.sync(ok); len(r.Rejected) != 0 || len(r.Plan) != 1 {
		t.Fatalf("metadata: %+v", r)
	}
}

func TestRulesValidation(t *testing.T) {
	e := setup(t)
	bad := 999
	acct := int64(bad)
	e.call("PUT", "/rules", []ruleJSON{{PathGlob: ""}}, http.StatusBadRequest, nil)
	e.call("PUT", "/rules", []ruleJSON{{PathGlob: "A", Backup: "maybe"}}, http.StatusBadRequest, nil)
	e.call("PUT", "/rules", []ruleJSON{{PathGlob: "A", TargetAccountID: &acct}}, http.StatusBadRequest, nil)
	var got []ruleJSON
	e.call("PUT", "/rules", []ruleJSON{{PathGlob: "/Documents/"}}, http.StatusOK, &got)
	if len(got) != 1 || got[0].PathGlob != "Documents" || got[0].Backup != "include" || got[0].Cleanup != "allowed" {
		t.Fatalf("defaults/normalisation: %+v", got)
	}
	e.call("POST", "/accounts/abc/full", nil, http.StatusBadRequest, nil)
	e.call("POST", "/accounts/424242/full", nil, http.StatusNotFound, nil)
	io.Discard.Write(nil)
}

func strPtr(s string) *string { return &s }
func f64(f float64) *float64  { return &f }
func intPtr(i int) *int       { return &i }
