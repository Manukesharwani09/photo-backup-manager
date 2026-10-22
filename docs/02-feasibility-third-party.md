# 02 — Feasibility: what Google, Android and other apps support

_Checked 2026-10-22 against official Google docs (sources at the bottom)._

## TL;DR
**It's feasible.** The one hard limit: since **31 March 2025**, the Google Photos API no longer lets an app list or read your whole library. It can only see media **that the app itself uploaded**. So:
- From the day the app starts uploading, it knows everything, because its own registry is the source of truth.
- Photos backed up **before** the app (for example by the Google Photos app on an old phone) are invisible to it. They need a **one-time baseline import** (see below).

## Google Photos Library API
| Need | Supported? | How / notes |
|---|---|---|
| Upload a photo or video to account X | ✅ | Scope `photoslibrary.appendonly`: upload the bytes → `mediaItems.batchCreate` (up to 50 per call). Stored at **original quality**, and it **counts against that account's storage**. |
| Several Google accounts | ✅ | One OAuth sign-in per account, with one refresh token stored per account. Nothing in the API limits this. |
| Read back / verify media the app uploaded | ✅ | Scope `photoslibrary.readonly.appcreateddata`: `mediaItems.get` / `list` returns only app-created items. |
| List or check the **existing** library (Google Photos app backups, old phone) | ❌ | `photoslibrary.readonly`, `photoslibrary.sharing` and `photoslibrary` were **removed on 31 Mar 2025**. |
| User picks existing photos by hand | ⚠️ | The **Picker API** lets the user choose items in a Google-hosted picker. It works for small manual matching, not for bulk reconciliation. |
| Albums that mirror phone folders | ✅ partly | The app can create albums and add **only its own uploads** (`photoslibrary.edit.appcreateddata` for `batchAddMediaItems`). Albums are **flat**, so nested folders become album titles such as `DCIM/Camera`. |
| Sharing albums | ❌ | Share and unshare methods now return 403. |
| Remaining storage on an account | ⚠️ | The Photos API has no way to read it. Option A: Drive API `about.get` → `storageQuota` (needs an extra Drive scope). Option B: detect the "storage full" error on upload, mark the account full, and ask the user to switch. |
| Delete a photo in the cloud | ❌ | The API has no delete. Cloud cleanup stays in the Google Photos app. This matters little, because cleanup here means **device** space. |
| Daily quota | ⚠️ | **10,000 requests per project per day** (media byte reads are separate, at 75k). Each upload is about 1 request, plus 1 `batchCreate` for every 50 files and album calls, so roughly **8–9k photos per day**. Daily use is fine, but the **first big upload takes several days**. Exceeding the quota returns `429`, so retry with backoff. |
| App publishing / OAuth | ⚠️ | Photos scopes are **sensitive**. For personal use: set up an OAuth consent screen and **publish it to "In production" while unverified**. Up to 100 users get a warning screen, and tokens **don't expire**. In **"Testing"** mode, refresh tokens **expire every 7 days**, so avoid it. A Play Store release would need Google OAuth verification and a privacy policy. |

## Android platform
| Need | Supported? | How |
|---|---|---|
| List all photos and videos with folder path | ✅ | `MediaStore` queries return `RELATIVE_PATH`, `DATE_TAKEN`, `SIZE` and `DISPLAY_NAME`. Permissions: `READ_MEDIA_IMAGES` / `READ_MEDIA_VIDEO` (Android 13+). |
| See WhatsApp media | ✅ | It is indexed by MediaStore under `Android/media/com.whatsapp/WhatsApp/Media/...` (Android 11+). `.Statuses` is hidden (`.nomedia`), which is fine. |
| Fingerprint files | ✅ | SHA-256 over the file stream. It takes time on first scan, so cache it keyed by (id, size, modified). |
| Delete local files (selective cleanup) | ✅ | Android 11+: `MediaStore.createDeleteRequest(uris)` shows **one system confirmation per batch**. Android 12+: the `MANAGE_MEDIA` special permission allows deleting with no prompt. |
| Background backup | ✅ | `WorkManager` with network (Wi-Fi) and charging constraints. Watch for new media with a `ContentObserver`. |

## What this means for each problem
| # | Problem | Verdict |
|---|---|---|
| 1 | Multiple accounts | ✅ |
| 2 | Know which account has which photos and time periods | ✅ for uploads made by the app. ⚠️ older backups need the baseline import. |
| 3 | Back up only new photos to account B without clearing the phone | ✅ The registry skips any fingerprint already recorded in any account. |
| 4 | Skip restored WhatsApp media, back up new | ✅ Restored files usually have **identical bytes**, so the SHA-256 matches. Fallbacks: filename pattern `IMG-YYYYMMDD-WA####`, size and date. Plus a **"restore cutoff" rule**: WhatsApp files that exist at first scan on the new phone are marked *known, skip*. |
| 5 | Documents → primary account, never cleaned | ✅ Folder rule plus a protected flag. Protected files are never added to a delete batch. |
| 6 | Per-folder cleanup | ✅ The user approves through the Android system dialog. |
| 7 | Keep folder structure | ⚠️ The full path is stored in the registry, and albums named after folders are created. Google Photos has no real nested folder tree. |

## Baseline import for photos backed up before the app
The API can't see them, so pick one approach (open question in doc 04):
1. **Google Takeout** (more accurate): export Google Photos from each account. Every photo comes with a `.json` sidecar holding the title, photo-taken time and URL. The app imports these and matches them to local files by **filename + taken-time (+ size)**, then records them as `baseline_assumed` for that account. The downside is that Takeout also downloads the media files, which can be large. The app could import only the JSON files.
2. **Manual declaration** (simpler): rules such as "folder `DCIM/Camera`, taken before 2026-06-30 → account A". It is fast but less exact.
3. **Do nothing**: treat everything as new, and let Google Photos' own duplicate detection absorb re-uploads into the **same** account. This doesn't help across accounts, and still costs quota.

## Existing apps / alternatives
- **PhotoSync, Autosync-style apps:** they upload "only new since last sync" to a target, but they keep **no registry across accounts**, have no WhatsApp-restore logic and no per-folder protected cleanup. So nothing ready-made solves all 7 problems.
- **rclone (Google Photos backend):** it has the same 2025 limit (it can only read what it uploaded). It is desktop-oriented.
- **Google Drive as the destination (fallback idea):** it keeps the **real folder tree**, and its full listing API would remove the "can't see old backups" problem. It shares the same 15 GB per account. It is not chosen, because the user wants Google Photos (timeline, search, faces), but it is worth keeping in mind.
- **Google Photos app's own backup:** must be **turned off** on this phone (or limited to folders the app doesn't manage). Otherwise two uploaders race each other and the registry becomes wrong.

## Sources
- Google Photos APIs updates (scope removal, 31 Mar 2025): https://developers.google.com/photos/support/updates
- Library API limits & quotas: https://developers.google.com/photos/library/legacy/guides/api-limits-quotas
