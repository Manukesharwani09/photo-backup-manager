// Command phonesim stands in for the Android app during the backend proof of concept.
// It scans a local folder laid out like a phone, calls POST /sync, uploads the planned files'
// bytes straight to Google Photos with the short-lived tokens the backend hands out, then
// reports the upload tokens with POST /uploads — the same flow the real app will use.
//
//	go run ./cmd/phonesim -dir testdata/phone
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
)

var mediaExt = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".heic": true, ".heif": true,
	".webp": true, ".gif": true, ".mp4": true, ".mov": true, ".3gp": true, ".mkv": true, ".webm": true}

func main() {
	server := flag.String("server", "http://localhost:8080", "backend URL")
	dir := flag.String("dir", "testdata/phone", "folder that plays the phone's storage")
	statePath := flag.String("state", "testdata/phonesim-device.json", "where the simulated device id is kept")
	dryRun := flag.Bool("dry-run", false, "sync only; don't upload")
	restore := flag.String("restore", "", "restore this folder from Google into -dir instead of backing up")
	flag.Parse()
	ctx := context.Background()

	deviceID, err := loadOrRegister(*server, *statePath)
	if err != nil {
		log.Fatalf("device: %v", err)
	}
	if *restore != "" {
		if err := restoreFolder(ctx, *server, deviceID, *restore, *dir); err != nil {
			log.Fatalf("restore: %v", err)
		}
		return
	}
	files, local, err := scan(*dir)
	if err != nil {
		log.Fatalf("scan: %v", err)
	}
	fmt.Printf("device %d: scanned %d media files in %s\n", deviceID, len(files), *dir)

	var plan struct {
		FirstScan bool             `json:"first_scan"`
		Counts    map[string]int   `json:"counts"`
		Plan      []planItem       `json:"plan"`
		Waiting   []map[string]any `json:"waiting"`
		Rejected  []string         `json:"rejected"`
		Tokens    map[string]struct {
			AccessToken string    `json:"access_token"`
			Expiry      time.Time `json:"expiry"`
		} `json:"tokens"`
	}
	if err := post(*server+"/sync", map[string]any{"device_id": deviceID, "files": files}, &plan); err != nil {
		log.Fatalf("sync: %v", err)
	}
	fmt.Printf("sync: first_scan=%v counts=%v planned=%d waiting=%d rejected=%d\n",
		plan.FirstScan, plan.Counts, len(plan.Plan), len(plan.Waiting), len(plan.Rejected))
	for _, w := range plan.Waiting {
		fmt.Printf("  waiting: file %v (%v)\n", w["file_id"], w["reason"])
	}
	for _, r := range plan.Rejected {
		fmt.Printf("  rejected: %s\n", r)
	}
	if *dryRun || len(plan.Plan) == 0 {
		return
	}

	var items []map[string]any
	for _, p := range plan.Plan {
		tok, ok := plan.Tokens[fmt.Sprint(p.AccountID)]
		if !ok {
			log.Printf("no token for account %d; skipping %s", p.AccountID, p.DisplayName)
			continue
		}
		path := local[p.RelativePath+"/"+p.DisplayName]
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("read %s: %v", path, err)
			continue
		}
		client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok.AccessToken, Expiry: tok.Expiry}))
		uploadToken, err := google.UploadBytes(ctx, client, p.DisplayName, mimeOf(p.DisplayName), data)
		if err != nil {
			log.Printf("upload %s: %v", p.DisplayName, err)
			continue
		}
		fmt.Printf("  uploaded bytes: %s/%s -> account %d (album %q)\n", p.RelativePath, p.DisplayName, p.AccountID, p.AlbumTitle)
		items = append(items, map[string]any{"file_id": p.FileID, "account_id": p.AccountID, "upload_token": uploadToken})
	}

	var done struct {
		Results []map[string]any `json:"results"`
	}
	if err := post(*server+"/uploads", map[string]any{"device_id": deviceID, "items": items}, &done); err != nil {
		log.Fatalf("uploads: %v", err)
	}
	for _, r := range done.Results {
		fmt.Printf("  result: file %v %v verified=%v %v %v\n", r["file_id"], r["status"], r["verified"], r["product_url"], r["message"])
	}
}

// restoreFolder downloads a folder's backed-up photos into dir at their original paths and reports
// the hashes of the bytes it received. The real app would also write GPS back into EXIF here.
func restoreFolder(ctx context.Context, server string, deviceID int64, folder, dir string) error {
	var plan struct {
		Items []struct {
			MediaID      int64    `json:"media_id"`
			RelativePath string   `json:"relative_path"`
			DisplayName  string   `json:"display_name"`
			AccountID    int64    `json:"account_id"`
			DownloadURL  string   `json:"download_url"`
			GPSLat       *float64 `json:"gps_lat"`
			GPSLng       *float64 `json:"gps_lng"`
		} `json:"items"`
		AlreadyOnDevice int      `json:"already_on_device"`
		MissingInGoogle []string `json:"missing_in_google"`
		Tokens          map[string]struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
	}
	if err := post(server+"/restore/plan", map[string]any{"device_id": deviceID, "folder": folder}, &plan); err != nil {
		return err
	}
	fmt.Printf("restore %s: %d to download, %d already on device, %d missing in Google\n",
		folder, len(plan.Items), plan.AlreadyOnDevice, len(plan.MissingInGoogle))
	var done []map[string]any
	for _, it := range plan.Items {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, it.DownloadURL, nil)
		req.Header.Set("Authorization", "Bearer "+plan.Tokens[fmt.Sprint(it.AccountID)].AccessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			return fmt.Errorf("download %s: %v %s", it.DisplayName, err, resp.Status)
		}
		target := filepath.Join(dir, filepath.FromSlash(it.RelativePath), it.DisplayName)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		gps := "none stored"
		if it.GPSLat != nil {
			gps = fmt.Sprintf("%.5f,%.5f (write back if missing)", *it.GPSLat, *it.GPSLng)
		}
		fmt.Printf("  restored %s/%s (%d bytes) gps: %s\n", it.RelativePath, it.DisplayName, len(data), gps)
		done = append(done, map[string]any{"media_id": it.MediaID, "relative_path": it.RelativePath,
			"display_name": it.DisplayName, "size": len(data), "sha256": hex.EncodeToString(sum[:])})
	}
	var res map[string]any
	if err := post(server+"/restore/done", map[string]any{"device_id": deviceID, "items": done}, &res); err != nil {
		return err
	}
	fmt.Printf("restore done: %v\n", res)
	return nil
}

type planItem struct {
	FileID       int64  `json:"file_id"`
	RelativePath string `json:"relative_path"`
	DisplayName  string `json:"display_name"`
	AccountID    int64  `json:"account_id"`
	AlbumTitle   string `json:"album_title"`
}

// scan walks dir like MediaStore would: one entry per media file with its folder, size, hash and metadata.
func scan(dir string) (files []map[string]any, local map[string]string, err error) {
	local = map[string]string{}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !mediaExt[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		rel, _ := filepath.Rel(dir, filepath.Dir(path))
		if rel == "." {
			rel = ""
		}
		rel = filepath.ToSlash(rel)
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		h := sha256.New()
		size, err := io.Copy(h, f)
		if err != nil {
			return err
		}
		info, _ := d.Info()
		entry := map[string]any{
			"relative_path": rel,
			"display_name":  d.Name(),
			"size":          size,
			"sha256":        hex.EncodeToString(h.Sum(nil)),
			"date_taken":    info.ModTime().UTC().Format(time.RFC3339),
			"source":        sourceOf(rel),
		}
		if _, err := f.Seek(0, io.SeekStart); err == nil {
			if cfg, _, err := image.DecodeConfig(f); err == nil {
				entry["width"], entry["height"] = cfg.Width, cfg.Height
			}
		}
		files = append(files, entry)
		local[rel+"/"+d.Name()] = path
		return nil
	})
	sort.Slice(files, func(i, j int) bool {
		return fmt.Sprint(files[i]["relative_path"], files[i]["display_name"]) < fmt.Sprint(files[j]["relative_path"], files[j]["display_name"])
	})
	return files, local, err
}

func sourceOf(folder string) string {
	f := strings.ToLower(folder)
	switch {
	case strings.Contains(f, "whatsapp"):
		return "whatsapp"
	case strings.Contains(f, "screenshot"):
		return "screenshot"
	case strings.Contains(f, "download"):
		return "download"
	case strings.HasPrefix(f, "dcim"):
		return "camera"
	}
	return "other"
}

func mimeOf(name string) string {
	if t := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); t != "" {
		return strings.Split(t, ";")[0]
	}
	return "application/octet-stream"
}

func loadOrRegister(server, statePath string) (int64, error) {
	var st struct {
		DeviceID int64 `json:"device_id"`
	}
	if b, err := os.ReadFile(statePath); err == nil {
		if err := json.Unmarshal(b, &st); err == nil && st.DeviceID > 0 {
			return st.DeviceID, nil
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}
	if err := post(server+"/devices", map[string]string{"model": "phonesim"}, &st); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return 0, err
	}
	b, _ := json.Marshal(st)
	return st.DeviceID, os.WriteFile(statePath, b, 0o644)
}

func post(url string, in, out any) error {
	b, _ := json.Marshal(in)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s: %s: %s", url, resp.Status, body)
	}
	return json.Unmarshal(body, out)
}
