# 03 — Architecture (draft)

There are **two services**:
- **Backend:** our server, the brain of the system. It holds accounts, tokens, the registry, rules, and every decision. **Current focus.**
- **App:** Android (Kotlin), the hands on the phone. It scans files, computes fingerprints, sends bytes and deletes files. **Built later**, once the backend proves every requirement can be met.

## Why the split looks like this
Only the phone can see and touch the photo files. Everything else, including the knowledge of what's where and what should happen, lives in the backend.

| Responsibility | Where | Why |
|---|---|---|
| Find files, read path / date / size, compute SHA-256 | App | Only the phone can read its storage (MediaStore). |
| Send photo bytes to Google | App, **directly to Google** | Avoids routing every photo through our server, which would double the bandwidth and need storage on our side. |
| Delete local files | App | Only the phone can delete them, and Android asks the user to confirm. |
| Google accounts, OAuth refresh tokens | Backend | Tokens stay on the server. The app gets only short-lived access tokens. |
| Registry (what is backed up where) | Backend DB | One source of truth. A new phone just logs in, and nothing needs exporting or importing. |
| Folder rules, account choice, "account full" handling | Backend | The decision engine. |
| `batchCreate`, albums, descriptions, verification | Backend | Server-side Google API calls using the same OAuth client. |
| Cleanup plan (what may be deleted) | Backend | The app only runs the plan the backend returns. |
| Dashboard data (per-account coverage) | Backend API | The app UI or a web page reads it. |

## Upload flow (end to end)
```
 App                              Backend                         Google Photos
  │ 1. POST /sync  (new files:       │                                 │
  │    path, size, date, sha256)    │                                 │
  │ ───────────────────────────────▶│ 2. check registry + rules       │
  │                                  │    → plan: file X → acct B,     │
  │ ◀───────────── upload plan ──── │      album "DCIM › Camera"      │
  │    + short-lived token (acct B) │                                 │
  │ 3. upload bytes ─────────────────────────────────────────────────▶│
  │ ◀──────────────────────────────────────────────── uploadToken ─── │
  │ 4. POST /uploads (sha256, token)│                                 │
  │ ───────────────────────────────▶│ 5. batchCreate (+description,   │
  │                                  │    album) ─────────────────────▶│
  │                                  │ ◀────────── mediaItemId ─────── │
  │                                  │ 6. save record, verify (get)    │
```
- Google upload tokens are valid for **1 day**, so the backend must call `batchCreate` before then. Whether a token created by the phone can be used by the backend is in the backend proof of concept below.
- If the app dies after step 3, the backend has no record yet, so the file stays pending and is simply sent again. If it dies after step 4, the backend completes steps 5–6 on its own.

## Data model (backend DB, first cut)
```
user(id, email, created_at)

device(id, user_id, model, registered_at, first_scan_at)   -- first_scan_at = "start point" per phone

account(id, user_id, email, role[primary|overflow], active, full_flag,
        refresh_token_enc, created_at)

media(id, user_id, sha256, phash NULL, width, height, date_taken,
      gps_lat NULL, gps_lng NULL)                           -- one row per unique photo

device_file(id, device_id, media_id, relative_path, display_name, size,
            source[camera|whatsapp|screenshot|download|other],
            state[existing_skip|pending|backed_up|excluded], present_on_device)

backup_record(id, media_id, account_id, gphotos_media_item_id, product_url, album_id,
              status[uploaded|verified|failed|deleted_in_cloud], uploaded_at, verified_at,
              last_checked_at)

folder_rule(id, user_id, path_glob, target_account_id NULL, backup[include|exclude],
            cleanup[allowed|protected], priority)

album(id, account_id, folder_path, title, gphotos_album_id)

account_period(id, account_id, started_at, ended_at NULL,
               end_reason[full|switched|manual] NULL)       -- when an account was the backup destination
```
**Key rule:** a `media` (sha256) is *backed up* if **any** `backup_record` exists for it with status `uploaded` or `verified`, across all accounts and devices. This solves #3 and the WhatsApp/new-phone case (#4).

Size: a hash is 32 bytes and a row is about 300 bytes, so 50k photos is about 15 MB per user.

## Same-photo detection (backend)
| Match | Action |
|---|---|
| SHA-256 equal (same bytes) | **Skip**, already backed up |
| Anything else | **Upload** |

Look-alikes with different bytes are uploaded, and the backend also groups them for the **"Similar photos"** feature:
- **How it groups them:** the app sends a perceptual hash (based on what the image looks like). The backend groups photos whose perceptual hashes are close, or whose filename, date taken, width × height and size match.
- **What the user can do:** review a group and delete local copies to free phone space.
- **Cloud copies:** the API can't delete them. Instead, the backend adds them to a **"🗑 To delete"** album in each account (all are our own uploads, so this is allowed). The app shows "Account A: 12 photos to delete → Open". One tap opens that album in Google Photos, where the user selects all and deletes. The scheduled cloud check then notices they're gone, updates the records, and removes the emptied album.

## Other flows
**First scan on a phone:** the app sends everything on the phone. The backend sets every file not already in the registry to `existing_skip`, and they are never uploaded. From then on, only new files are backed up.

**New phone / WhatsApp restore:** the new device registers and sends its first scan. Restored files either match a known sha256 (already backed up) or become `existing_skip`. WhatsApp files that arrive later are backed up as usual.

**Account full:** `batchCreate` (or the upload) returns a storage error. The backend sets `full_flag`, picks the next overflow account (or asks the user), and the next plan points there. Pending files only. The current `account_period` is closed (`ended_at = now`, `end_reason = full`), and a new one is opened for the next account. A manual switch does the same with `end_reason = switched` / `manual`.

**Time periods per account:** two different things are tracked.
1. **Active period:** when the account was the main backup destination. It's recorded in `account_period`.
2. **Photo dates:** which `date_taken` range the account holds. This is calculated from `media.date_taken` joined to `backup_record`, so nothing extra is stored.

They don't always line up:
- Rule-based folders (Documents → primary) keep filling the primary account after its active period ends.
- An old photo received today goes to the current account.

So "where are my March 2024 photos?" searches by `date_taken`, and the answer can list more than one account.

Dashboard example:
```
Account A   active 10 Jan 2026 → 15 Jun 2026 (full)
            photos taken: Mar 2019 → Jun 2026 · 8,412 items
            folders: DCIM/Camera, WhatsApp/Images, Documents/Bills
Account B   active 15 Jun 2026 → now
            photos taken: Jan 2022 → Oct 2026 · 2,105 items
```

**Cloud check (scheduled):** the Google Photos API has no delete, but you can delete photos yourself in the Google Photos app. So the backend regularly runs `mediaItems.batchGet` (50 per call) on its records:
- If an item comes back "not found", its record is set to `deleted_in_cloud`, and that photo no longer counts as backed up.
- If the photo is still on the phone, it becomes `pending` again.
- If it isn't on the phone anymore, it's flagged on the dashboard.

**Selective cleanup:** the app requests a cleanup plan for the chosen folders. The backend returns the files that are `verified`, in folders with `cleanup = allowed`, and not protected. The app shows a preview, the user confirms, the app deletes, then reports back with `present_on_device = false`.

**Restore folder (first version):**
1. You pick a folder.
2. The backend fetches fresh `baseUrl`s for that folder's items from every account. The links expire after about 60 minutes, so they're fetched just before download.
3. It returns them with the original paths and GPS.
4. The app downloads each one (`=d` for photos, `=dv` for videos) into the same `relative_path`.
5. It writes GPS back into EXIF if Google removed it.
6. It reports the restored file's new hash, which the backend links to the same `media`, so it's never uploaded again.

Media downloads have their own quota of 75k per day.

## Backend API (first cut)
| Endpoint | Purpose |
|---|---|
| `POST /accounts/connect` → OAuth callback | Add a Google account; store the encrypted refresh token |
| `GET /accounts` | Accounts, roles, full flag, coverage |
| `POST /devices` | Register a phone |
| `POST /sync` | App sends new or changed file metadata and hashes; returns the upload plan |
| `POST /uploads` | App reports uploadTokens; backend runs `batchCreate`, album and verify |
| `GET/PUT /rules` | Folder rules |
| `POST /cleanup/plan` | Files that may be deleted from chosen folders |
| `POST /cleanup/done` | App reports deleted files |
| `GET /dashboard` | Per-account active periods, photo date ranges, counts, folders, remaining space; "where is this photo?" |
| `GET /photos?taken_from=&taken_to=` | Which accounts hold photos taken in a period |

## Points to note
- **We hold users' Google tokens.** Encrypt them at rest. That's fine for personal use, and it is a privacy-policy item if published.
- **The quota is per Google Cloud project**, so it's shared by every user of our backend (10k requests per day).
- **Phone photos never pass through or get stored on our server.** Only metadata and hashes do.
