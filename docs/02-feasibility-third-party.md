# 02 — Feasibility: what Google, Android and other apps support

_Checked 2026-10-22 against official Google docs (sources at the bottom). Updated 2026-10-24 after the scope decisions in doc 01._

## TL;DR
**It's feasible, and all 7 problems are covered.**

Since **31 March 2025**, the Google Photos API lets an app see only the media **that the app itself uploaded**. That doesn't affect us: we only back up **new photos from the day the app starts**, and every one of them goes through our app. Our registry knows about every upload, and Google can confirm each one.

## Google Photos Library API
| Need | Supported? | How / notes |
|---|---|---|
| Upload a photo or video to account X | ✅ | Scope `photoslibrary.appendonly`: upload the bytes → `mediaItems.batchCreate` (up to 50 per call). Stored at **original quality** (what we want), and it counts against that account's storage. |
| Several Google accounts | ✅ | One OAuth sign-in per account, with one refresh token stored per account. Nothing in the API limits this. |
| Read back / verify media the app uploaded | ✅ | Scope `photoslibrary.readonly.appcreateddata`: `mediaItems.get` / `list` returns only app-created items, which is all we need. |
| Download our own uploads (for restore) | ✅ | The `baseUrl` + `=d` of an app-created item returns the file. ⚠️ Location (GPS) data **may be removed** from the downloaded copy. To be confirmed in the test app; workaround in doc 03. |
| List the library from before the app | ❌ | `photoslibrary.readonly` was removed on 31 Mar 2025. **Not needed:** we don't back up or track photos from before the app starts. |
| One album per phone folder | ✅ | The app creates albums and adds its own uploads (`photoslibrary.edit.appcreateddata` for `batchAddMediaItems`). Example names: `DCIM › Camera`, `Documents › Bills`. Each album holds that folder's photos. Sorting albums by title in Google Photos keeps them grouped. Flat albums are fine for us. |
| Original path on each photo | ✅ | The `description` field (up to 1,000 characters) is set at `batchCreate`. We put the original path there, and it shows in Google Photos' info panel. Whether Google Photos search finds it is to be confirmed in the test app. |
| Remaining storage on an account | ⚠️ | The Photos API has no way to read it. Option A: Drive API `about.get` → `storageQuota` (needs a Drive scope, open question). Option B: detect the "storage full" error on upload, mark the account full, and ask the user to switch. |
| Resumable upload | ✅ | A large video interrupted halfway continues from where it stopped instead of starting over. |
| Daily quota | ✅ for personal use | **10,000 requests per project per day.** Each upload is about 1 request, plus 1 `batchCreate` for every 50 files and album calls, so roughly 8–9k photos per day. More than enough for personal use. A wider release would need a quota increase. |
| App publishing / OAuth | ✅ for personal use | Photos scopes are **sensitive**. Set up an OAuth consent screen and publish it to **"In production" while unverified**. Up to 100 users get a warning screen, and tokens don't expire. Avoid **"Testing"** mode, where refresh tokens expire every 7 days. |

## Android platform
| Need | Supported? | How |
|---|---|---|
| List all photos and videos with folder path | ✅ | `MediaStore` queries return `RELATIVE_PATH`, `DATE_TAKEN`, `SIZE` and `DISPLAY_NAME`. Permissions: `READ_MEDIA_IMAGES` / `READ_MEDIA_VIDEO` (Android 13+). |
| See WhatsApp media | ✅ | It is indexed by MediaStore under `Android/media/com.whatsapp/WhatsApp/Media/...` (Android 11+). |
| Fingerprint files | ✅ | SHA-256 over the file bytes. Hash once and cache it keyed by (id, size, modified). |
| Delete local files (selective cleanup) | ✅ | Android 11+: `MediaStore.createDeleteRequest(uris)` shows one system confirmation per batch. Android 12+: the "Media management" (`MANAGE_MEDIA`) permission allows deleting with no prompt. |
| Background backup | ✅ | `WorkManager` with network and charging constraints, plus a "Backing up…" notification while uploading. If the phone's battery saver kills the app, the next run continues from the next pending file, because the registry tracks each file's status. Nothing is lost or uploaded twice. |

## What this means for each problem
| # | Problem | Verdict |
|---|---|---|
| 1 | Multiple accounts | ✅ |
| 2 | Know which account has which photos and time periods | ✅ Every upload is recorded in the registry. |
| 3 | Back up only new photos to account B without clearing the phone | ✅ The registry skips any fingerprint already recorded in any account. |
| 4 | Skip restored WhatsApp media, back up new | ✅ Files present at first scan are skipped. Restored files usually have identical bytes, so if the registry was moved over, their fingerprints match too. |
| 5 | Documents → primary account, never cleaned | ✅ Folder rule plus a protected flag. The folder holds photos, so Google Photos accepts it. |
| 6 | Per-folder cleanup | ✅ The user approves through the Android system dialog. |
| 7 | Keep folder grouping | ✅ One album per folder, the original path in each description, the folder view in our app, and optional restore back into the same folders. |

## Existing apps / alternatives
- **PhotoSync, Autosync-style apps:** they upload "only new since last sync" to a target, but they keep **no registry across accounts**, have no WhatsApp-restore logic and no per-folder protected cleanup. So nothing ready-made solves all 7 problems.
- **rclone (Google Photos backend):** it has the same 2025 limit. It is desktop-oriented.
- **Google Photos app's own backup:** must be **turned off** for folders our app manages. Otherwise two uploaders race each other. Note that it won't show our uploads as "backed up" (no cloud icon on local photos).

## Remaining outside risk
Google already cut this API in 2025, and its docs now sit under a "legacy" path. Uploading survived, but Google could restrict it further. Keeping our own registry, with a backup copy, limits the damage.

## Sources
- Google Photos APIs updates (scope removal, 31 Mar 2025): https://developers.google.com/photos/support/updates
- Library API limits & quotas: https://developers.google.com/photos/library/legacy/guides/api-limits-quotas
