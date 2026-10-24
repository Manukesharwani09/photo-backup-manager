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
              status[uploaded|verified|failed], uploaded_at, verified_at)

folder_rule(id, user_id, path_glob, target_account_id NULL, backup[include|exclude],
            cleanup[allowed|protected], priority)

album(id, account_id, folder_path, title, gphotos_album_id)
```
**Key rule:** a `media` (sha256) is *backed up* if **any** `backup_record` exists for it with status `uploaded` or `verified`, across all accounts and devices. This solves #3 and the WhatsApp/new-phone case (#4).

Size: a hash is 32 bytes and a row is about 300 bytes, so 50k photos is about 15 MB per user.

## Same-photo detection (backend)
| Match | Confidence | Action |
|---|---|---|
| SHA-256 equal | Certain | Skip automatically |
| Filename + date taken + width × height + roughly the size equal | High | Skip, marked "assumed" |
| Perceptual hash close (sent by the app) | Medium | Ask the user |
| No match | — | Upload |

## Other flows
**First scan on a phone:** the app sends everything on the phone. The backend sets every file not already in the registry to `existing_skip`, and they are never uploaded. From then on, only new files are backed up.

**New phone / WhatsApp restore:** the new device registers and sends its first scan. Restored files either match a known sha256 (already backed up) or become `existing_skip`. WhatsApp files that arrive later are backed up as usual.

**Account full:** `batchCreate` (or the upload) returns a storage error. The backend sets `full_flag`, picks the next overflow account (or asks the user), and the next plan points there. Pending files only.

**Selective cleanup:** the app requests a cleanup plan for the chosen folders. The backend returns the files that are `verified`, in folders with `cleanup = allowed`, and not protected. The app shows a preview, the user confirms, the app deletes, then reports back with `present_on_device = false`.

**Restore folder (optional):** the backend gives the app the `baseUrl=d` download links for our uploads plus the original paths and GPS. The app writes the files back into the same folders and restores the GPS in EXIF if Google removed it.

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
| `GET /dashboard` | Per-account counts, date ranges, folders; "where is this photo?" |

## Points to note
- **We hold users' Google tokens.** Encrypt them at rest. That's fine for personal use, and it is a privacy-policy item if published.
- **The quota is per Google Cloud project**, so it's shared by every user of our backend (10k requests per day).
- **Phone photos never pass through or get stored on our server.** Only metadata and hashes do.
