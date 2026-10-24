# 04 — Decisions, open questions & next step

## Decided
| Topic | Decision |
|---|---|
| Structure | Two services: **backend** (the brain: accounts, registry, rules, Google API) and **Android app** (scan, hash, upload bytes, delete). |
| Current focus | **Backend first.** Prove the system can meet every requirement; build the app after. |
| Scope | Only new photos from the day the app starts. Everything already on the phone at the first scan is skipped. No baseline import, no Takeout. |
| Quality | Original quality always (the API default) |
| Documents folder | Photos only → goes to Google Photos, primary account, protected from cleanup |
| Folder grouping | One flat album per phone folder, named by its path (`DCIM › Camera`). No nested albums needed. |
| Daily quota (10k requests) | Enough for personal use |
| Background uploads | Handled by the app later. If it's killed, the next run continues from the next pending file. Not an issue. |
| Viewing after cleanup | Open the photo in Google Photos for the right account. The backend knows which account holds it. |
| Backend stack | **Go + PostgreSQL** |
| Hosting | Left to us. Proof of concept runs **locally** (Docker Postgres). Once it works: **Google Cloud Run + Neon Postgres** (both have free tiers; we already need a Google Cloud project for OAuth). A custom domain only once the project works. |
| Google Photos app's own backup | **Off completely** on the phone. Our app is the only uploader. |
| Storage-left display | **Show remaining space per account.** Uses Drive `about.get` → `storageQuota`, with the narrowest Drive scope that allows it (to be confirmed in the proof of concept; `drive.file` is expected to work). The figure is the account's total free space, which Gmail, Drive and Photos share. The "account full" error is still handled as a fallback. |
| Restore-to-folder | **In the first version.** Pick a folder, and the app downloads its photos from every account back into the same phone folder. The backend stores GPS at upload, so the app can write it back if Google removes it from the download. |
| Same-photo rule | **Only an exact SHA-256 match is skipped.** Anything with different bytes is uploaded, even if it looks the same (WhatsApp re-send, edit). |
| Similar photos | **A "Similar photos" feature** groups look-alikes using the perceptual hash, plus name, date and size. The user can review a group and free phone space by deleting local copies. Cloud duplicates can't be deleted through the API, so the app opens them in Google Photos for manual removal. |

## Still open
None right now. The next step is the backend proof of concept.

## Backend proof of concept (next step, not built yet)
No phone needed. A **script stands in for the app**, using a local folder of test photos with sub-folders.
1. **OAuth:** connect **2 Google accounts** to the backend (scopes `photoslibrary.appendonly`, `photoslibrary.readonly.appcreateddata`, `photoslibrary.edit.appcreateddata`). Store refresh tokens.
2. **Sync:** the script sends file metadata and SHA-256 for the folder. The backend returns an upload plan.
3. **Split upload:** the script uploads bytes to Google with a short-lived access token from the backend, then sends the uploadTokens to the backend. **Check: can the backend `batchCreate` with a token the "phone" created?**
4. **Albums and descriptions:** the backend creates one album per folder (`DCIM › Camera`, `Documents › Bills`) and sets the description = original path.
5. **Verify:** `mediaItems.get` / `list` with app-created access.
6. **No re-upload:** run the sync again, plus a copy of a photo under another name. Both must be skipped (same hash).
7. **Account switch:** mark account A full. New files must go to account B, and nothing from A is re-sent.
8. **First-scan rule:** files present before the "start point" become `existing_skip` and are never uploaded.
9. **Cleanup plan:** returns only verified, non-protected files from allowed folders (Documents excluded).
10. **Restore:** download one album's photos with `baseUrl=d` (`=dv` for videos) into a local folder with the original paths. Write GPS back if it was removed. Link the restored file's new hash to the same item.
11. **Similar photos:** a re-compressed copy of a test photo is uploaded (different bytes) and grouped with the original in the "similar" results.
12. **Remaining space:** `about.get` → `storageQuota` per account, using the narrowest Drive scope that works.
13. **Checks:**
    - Does `baseUrl=d` download remove GPS?
    - Does Google Photos search find the description?
    - Do motion photos and HEIC files survive the upload?
    - What error does Google return when an account is full?

If all of these pass, every requirement is proven on the backend side, and the app becomes ordinary Android work: MediaStore scan, hashing, WorkManager, delete requests and UI.

## If publishing later (not needed now)
- Google OAuth verification for the sensitive Photos scopes, plus a privacy policy (we store users' tokens and photo metadata).
- Play Store review of the media and "Media management" permissions.
- A request for a higher Photos API quota, since the 10k per day is shared by all users.
