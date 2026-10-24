# Status

_Last updated: 2026-10-24. **Update this file whenever work lands.** Add the dated entry to [worklog.md](worklog.md) too._

## Phase
**Backend proof of concept.** The goal is to prove on the backend side that every requirement in [../product/requirements.md](../product/requirements.md) is achievable with the Google APIs, before building the Android app. A script stands in for the phone, using a local `testdata/` folder.

Legend: ✅ done · 🟡 built, not yet verified · ⬜ not started

## Proof-of-concept checklist
| # | Step | Status | Notes |
|---|---|---|---|
| 0 | Google Cloud project, consent screen, scopes, test users, OAuth client, `.env` | ✅ | [../setup/google-cloud.md](../setup/google-cloud.md) |
| 0b | Go skeleton, DB schema (9 tables), migrations, config, token encryption | ✅ | [../architecture/code-layout.md](../architecture/code-layout.md) |
| 1 | **OAuth:** connect 2 accounts (primary + overflow), store encrypted refresh tokens, open `account_period` | ✅ | Verified 2026-10-24 with real accounts: `mkesharwani125@gmail.com` (primary, id 1, period open) and `tanyamishra12346789@gmail.com` (overflow, id 2). Tokens stored encrypted |
| 13 | **Remaining space** per account (Drive `storageQuota`) | ✅ | Verified: `drive.file` is enough. Primary 12.08 GB free of 15 GB; overflow 14.96 GB free |
| 2 | **Sync:** stand-in phone sends metadata + SHA-256; backend returns an upload plan | ✅ | `POST /devices`, `POST /sync`; `cmd/phonesim` is the stand-in phone. Live run 2026-10-24 |
| 3 | **Split upload:** phone uploads bytes with a backend-issued access token; backend runs `batchCreate` | ✅ | **A2 verified with real Google:** 3 photos uploaded to `mkesharwani125` and verified. `POST /uploads` |
| 4 | **Albums + descriptions:** one album per folder (`DCIM › Camera`); description = original path | ✅ | Live: albums `DCIM › Camera` and `Documents › Bills` created. Whether search finds the description is still A4 |
| 5 | **Verify** uploaded items (`mediaItems.get`) | ✅ | `mediaItems:batchGet` right after creating; live records are `verified` |
| 6 | **No re-upload:** re-sync + renamed copy, both skipped | ✅ | Live: the re-sync planned 0; the byte-identical copy in `Pictures/Saved` counts as backed up. Also enforced by a unique DB index (one live backup per photo) |
| 7 | **Account switch:** mark A full; new files go to B; nothing from A re-sent; periods updated | ✅ (tests) | Covered end-to-end with a fake Google (`TestFullBackupFlow`, `TestStorageFullDuringUpload`). **Deliberately not run live**, so the real primary isn't left flagged full. `POST /accounts/{id}/full` |
| 8 | **First-scan rule:** files present at the start point become `existing_skip` | ✅ | Live: the 2 start-point photos were never uploaded. Editing such a file later (new bytes) makes it pending |
| 9 | **Cleanup plan:** only verified, non-protected files in allowed folders | ✅ | `POST /cleanup/plan`, `/cleanup/done`. Live preview: offered only the verified camera photos and the duplicate; kept the Documents bill (protected) and the start-point and excluded photos |
| 10 | **Restore:** download an album into the original paths; write GPS back; link the new hash | ✅ | Live: restored `DCIM` to a second simulated phone from real Google; both files linked, no re-upload. **A3 verified: Google strips GPS and changes the bytes on download.** GPS write-back is the app's job (phonesim doesn't parse EXIF) |
| 11 | **Similar photos:** a re-compressed copy is uploaded and grouped with the original | ✅ (tests) | `GET /similar`. Tested with a fake Google. Live data has no `phash` yet (phonesim doesn't compute one), so live grouping isn't exercised |
| 12 | **"To delete" album** + scheduled cloud check marks `deleted_in_cloud` | ✅ | Live: the "🗑 To delete" album was created in `mkesharwani125` and a test photo added (the `edit.appcreateddata` scope works). The cloud check confirmed 5/5 records present. Deletion handling is tested with a fake Google. **Correction: albums can't be deleted via the API**, so the emptied album stays and is reused |
| 15 | **Dashboard + photos by date** (problem #2) | ✅ | `GET /dashboard`, `GET /photos/where`. Live: 3 items in primary, 1 waiting in To delete, alert shown |
| 14 | Remaining checks: description search, motion/HEIC, account-full error | ⬜ | A4–A6. A4 and A5 need a real phone or manual checks in Google Photos; A6 needs a nearly full account |

If every row reaches ✅, the backend side is proven, and the Android app becomes ordinary work: MediaStore scan, hashing, WorkManager, delete requests and UI.

## Test coverage (what is and isn't tested)
Run with: see [../setup/local-dev.md](../setup/local-dev.md#tests). All passing on 2026-10-24.

| Area | Edge cases covered by automated tests |
|---|---|
| `crypto` | round-trip; fresh nonce per seal; tampered data, wrong key, short input and bad key length are all rejected |
| `config` | `.env` parsing (comments, blanks, spaces, junk lines); base64 values ending in `=`; real env wins over `.env`; missing file OK; missing or invalid client/key errors |
| `google` | missing-scope detection (user unticked a checkbox; `openid` not required); auth URL has offline access, consent, state, redirect, all scopes; unlimited quota |
| `store` (DB) | migrations idempotent; default user stable; first account opens one period; reconnect refreshes token without a new row or period; **only one primary** (second primary rejected and not saved; same primary may reconnect); invalid role rejected |
| `plan` | folder normalisation; album titles; rule glob matching (sub-folders, `*` one level, `**`, case-insensitive, anchored); rule precedence (priority > specificity > id); account choice (current, overflow when full, back to primary, none left, fixed target never falls back: full, disabled, missing) |
| `google` Photos | upload headers and token; empty or failed upload; batchCreate result mapping (a "success" with no id is a failure); description trimmed to 1000 characters; batch-size limits; batchGet reports missing items; album creation; storage-full detection |
| `api` sync/upload (DB + fake Google) | **full flow**: start point → rules → plan (dedupe of identical bytes, excluded folder, per-folder albums, per-account tokens) → upload → verify → re-sync plans nothing → re-upload refused → primary full: camera goes to overflow and the period switches, Documents wait (`target_account_full`) → all full (`no_account_available`). **Storage full during upload**: account flagged, files stay pending and go to overflow next sync. **Upload edge cases**: Google rejects a token, empty token, other device's file, duplicate entries, unknown account. **Sync validation**: unknown device, unknown JSON field, bad hash, empty name, bad source, bad date, out-of-range or half GPS; an unchanged start-point file stays skipped, an edited one is uploaded. **Rules validation** |
| `api` cleanup | only verified + unprotected + on-phone files offered; sub-folders included; protected folder yields nothing; reasons counted; `/cleanup/done` ignores bogus, unbacked and unverified ids; removed files not offered again |
| `api` restore | new phone gets links, paths and GPS from every account; old phone skips files it has; restored bytes (different hash) linked, so the first and later syncs upload nothing; dashboard doesn't double-count restored copies; items deleted in Google reported; bad hash and unknown media rejected |
| `api` similar / To delete / cloud check | look-alikes all uploaded then grouped (transitive, threshold boundary, same name+date+size); To delete album created once and reused; non-cloud ids counted; cloud check: present items touched, **deliberately deleted copy never re-uploaded**, **accidentally deleted photo re-queued**, Google outage never treated as deletion |
| `api` dashboard | per-account items, date range, folders and periods across an account switch; "March" spans both accounts (late arrival); inclusive end date; undated count; bad query params; alerts (full, none available, To delete waiting, Google unreachable → alert not failure) |
| `api` | bad role → 400; start redirects to Google with a strong state that remembers the role; forged, expired and reused states rejected; Google `error=` surfaced; abandoned states cleaned up |

**Not covered yet:**
- The real Google flow (code exchange, userinfo email, Drive quota) has no automated tests; it was verified manually on 2026-10-24.
- Row 14 (A4–A6): needs manual checks or a real phone.
- Live similar-photo grouping (phonesim sends no `phash`).
- Live account switch (row 7), on purpose; see the row note.

## Next action
The backend proof of concept covers every requirement (rows 0–13 and 15). Remaining:
1. **A4–A6 manual checks:**
   - A4: in Google Photos, search for `DCIM/Camera`. Does the description match?
   - A5: upload a real motion photo and a HEIC from a phone.
   - A6: the exact account-full error.
2. **Hardening before the app depends on it:** auth between app and backend (today any local caller is trusted), bucketing for similar photos, and pagination for large syncs.
3. The Android app can start against the API above. See [../app/design-brief.md](../app/design-brief.md).

## Environment snapshot
- Server: `./bin/server` on `http://localhost:8080` (local, run manually).
- DB: local Postgres 18, database `photobackup`.
- Live test data: `testdata/phone` = device 1, `testdata/newphone` = device 2 (restore test). Both are git-ignored.
- In `mkesharwani125`'s Google Photos: 3 test photos in albums `DCIM › Camera` and `Documents › Bills`, plus a "🗑 To delete" album holding one of them. All can be deleted by hand.
- Rules in the live DB: `Documents` → account 1 (protected); `Private` excluded.
- Google Cloud: project `photo-backup-manager`. Test users: `mkesharwani125@gmail.com`, `tanyamishra12346789@gmail.com`, `1by22cs111@bmsit.in`.
