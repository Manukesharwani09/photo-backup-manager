package google

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakePhotos(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	old := PhotosAPI
	PhotosAPI = srv.URL
	t.Cleanup(func() { PhotosAPI = old })
}

func TestUploadBytesSendsHeadersAndReturnsToken(t *testing.T) {
	fakePhotos(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/uploads" || r.Header.Get("X-Goog-Upload-Protocol") != "raw" ||
			r.Header.Get("X-Goog-Upload-Content-Type") != "image/jpeg" || r.Header.Get("X-Goog-Upload-File-Name") != "a.jpg" {
			t.Errorf("unexpected request %s %v", r.URL.Path, r.Header)
		}
		b, _ := io.ReadAll(r.Body)
		if string(b) != "bytes" {
			t.Errorf("body %q", b)
		}
		io.WriteString(w, "TOKEN123\n")
	})
	tok, err := UploadBytes(context.Background(), http.DefaultClient, "a.jpg", "image/jpeg", []byte("bytes"))
	if err != nil || tok != "TOKEN123" {
		t.Fatalf("got %q %v", tok, err)
	}
}

func TestUploadBytesErrors(t *testing.T) {
	fakePhotos(t, func(w http.ResponseWriter, r *http.Request) { http.Error(w, "nope", http.StatusForbidden) })
	if _, err := UploadBytes(context.Background(), http.DefaultClient, "a", "image/jpeg", nil); err == nil {
		t.Fatal("expected error on 403")
	}
	fakePhotos(t, func(w http.ResponseWriter, r *http.Request) {})
	if _, err := UploadBytes(context.Background(), http.DefaultClient, "a", "image/jpeg", nil); err == nil {
		t.Fatal("expected error on empty token")
	}
}

func TestBatchCreateMapsResultsAndTruncatesDescription(t *testing.T) {
	fakePhotos(t, func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			AlbumID       string `json:"albumId"`
			NewMediaItems []struct {
				Description     string `json:"description"`
				SimpleMediaItem struct {
					UploadToken string `json:"uploadToken"`
				} `json:"simpleMediaItem"`
			} `json:"newMediaItems"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		if in.AlbumID != "alb" || len(in.NewMediaItems) != 3 {
			t.Errorf("bad request %+v", in)
		}
		if n := len([]rune(in.NewMediaItems[0].Description)); n != MaxDescriptionLen {
			t.Errorf("description length %d", n)
		}
		io.WriteString(w, `{"newMediaItemResults":[
			{"uploadToken":"t1","status":{"message":"Success"},"mediaItem":{"id":"m1","productUrl":"https://photos/m1"}},
			{"uploadToken":"t2","status":{"code":3,"message":"Failed: bad token"}},
			{"uploadToken":"t3","status":{"message":"OK"}}]}`)
	})
	long := strings.Repeat("é", MaxDescriptionLen+5)
	res, err := BatchCreate(context.Background(), http.DefaultClient, "alb", []NewItem{
		{UploadToken: "t1", Description: long}, {UploadToken: "t2"}, {UploadToken: "t3"}})
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Code != 0 || res[0].MediaItemID != "m1" || res[0].ProductURL == "" {
		t.Errorf("item 1: %+v", res[0])
	}
	if res[1].Code != 3 {
		t.Errorf("item 2: %+v", res[1])
	}
	if res[2].Code == 0 { // success status but no item id must not count as success
		t.Errorf("item 3: %+v", res[2])
	}
}

func TestBatchCreateLimits(t *testing.T) {
	if res, err := BatchCreate(context.Background(), http.DefaultClient, "", nil); err != nil || res != nil {
		t.Fatal("empty batch should be a no-op")
	}
	if _, err := BatchCreate(context.Background(), http.DefaultClient, "", make([]NewItem, MaxBatch+1)); err == nil {
		t.Fatal("oversized batch accepted")
	}
}

func TestBatchGetReportsMissing(t *testing.T) {
	fakePhotos(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query()["mediaItemIds"]; len(got) != 2 {
			t.Errorf("ids %v", got)
		}
		io.WriteString(w, `{"mediaItemResults":[{"mediaItem":{"id":"a"}},{"status":{"code":5,"message":"not found"}}]}`)
	})
	found, err := BatchGet(context.Background(), http.DefaultClient, []string{"a", "gone"})
	if err != nil {
		t.Fatal(err)
	}
	if !found["a"] || found["gone"] {
		t.Fatalf("got %v", found)
	}
}

func TestCreateAlbum(t *testing.T) {
	fakePhotos(t, func(w http.ResponseWriter, r *http.Request) {
		var in map[string]map[string]string
		json.NewDecoder(r.Body).Decode(&in)
		if in["album"]["title"] != "DCIM › Camera" {
			t.Errorf("title %q", in["album"]["title"])
		}
		io.WriteString(w, `{"id":"alb1","title":"DCIM › Camera"}`)
	})
	id, err := CreateAlbum(context.Background(), http.DefaultClient, "DCIM › Camera")
	if err != nil || id != "alb1" {
		t.Fatalf("got %q %v", id, err)
	}
}

func TestIsStorageFull(t *testing.T) {
	cases := []struct {
		code int
		msg  string
		want bool
	}{
		{8, "", true},
		{13, "Not enough storage to upload", true},
		{3, "Failed: Quota exceeded for account", true},
		{3, "Failed: invalid upload token", false},
		{0, "", false},
	}
	for _, c := range cases {
		if got := IsStorageFull(c.code, c.msg); got != c.want {
			t.Errorf("IsStorageFull(%d, %q) = %v", c.code, c.msg, got)
		}
	}
}
