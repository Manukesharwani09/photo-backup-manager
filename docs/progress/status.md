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
| 1 | **OAuth:** connect 2 accounts (primary + overflow), store encrypted refresh tokens, open `account_period` | 🟡 | `/oauth/start`, `/oauth/callback`. Unit and DB tests pass (see Test coverage). Real sign-in with both accounts still to do (waiting for the user) |
| 13 | **Remaining space** per account (Drive `storageQuota`) | 🟡 | Part of `GET /accounts`. Verifies A1 in [../open-questions.md](../open-questions.md) |
| 2 | **Sync:** stand-in phone sends metadata + SHA-256; backend returns an upload plan | ⬜ | |
| 3 | **Split upload:** phone uploads bytes with a backend-issued access token; backend runs `batchCreate` | ⬜ | Verifies A2 (the key unknown) |
| 4 | **Albums + descriptions:** one album per folder (`DCIM › Camera`); description = original path | ⬜ | |
| 5 | **Verify** uploaded items (`mediaItems.get`) | ⬜ | |
| 6 | **No re-upload:** re-sync + renamed copy, both skipped | ⬜ | |
| 7 | **Account switch:** mark A full; new files go to B; nothing from A re-sent; periods updated | ⬜ | |
| 8 | **First-scan rule:** files present at the start point become `existing_skip` | ⬜ | |
| 9 | **Cleanup plan:** only verified, non-protected files in allowed folders | ⬜ | |
| 10 | **Restore:** download an album into the original paths; write GPS back; link the new hash | ⬜ | Verifies A3 |
| 11 | **Similar photos:** a re-compressed copy is uploaded and grouped with the original | ⬜ | |
| 12 | **"To delete" album** + scheduled cloud check marks `deleted_in_cloud` | ⬜ | |
| 14 | Remaining checks: description search, motion/HEIC, account-full error | ⬜ | A4–A6 |

If every row reaches ✅, the backend side is proven, and the Android app becomes ordinary work: MediaStore scan, hashing, WorkManager, delete requests and UI.

## Test coverage (what is and isn't tested)
Run with: see [../setup/local-dev.md](../setup/local-dev.md#tests). 26 tests (30 including sub-tests), all passing on 2026-10-24.

| Area | Edge cases covered by automated tests |
|---|---|
| `crypto` | round-trip; fresh nonce per seal; tampered data, wrong key, short input and bad key length are all rejected |
| `config` | `.env` parsing (comments, blanks, spaces, junk lines); base64 values ending in `=`; real env wins over `.env`; missing file OK; missing or invalid client/key errors |
| `google` | missing-scope detection (user unticked a checkbox; `openid` not required); auth URL has offline access, consent, state, redirect, all scopes; unlimited quota |
| `store` (DB) | migrations idempotent; default user stable; first account opens one period; reconnect refreshes token without a new row or period; **only one primary** (second primary rejected and not saved; same primary may reconnect); invalid role rejected |
| `api` | bad role → 400; start redirects to Google with a strong state that remembers the role; forged, expired and reused states rejected; Google `error=` surfaced; abandoned states cleaned up |

**Not covered yet:**
- The real Google flow: code exchange, userinfo email, Drive quota. That needs the manual sign-in.
- Everything in rows 2–14, which isn't built yet.

## Next action
1. The user connects `mkesharwani125@gmail.com` (primary) and `tanyamishra12346789@gmail.com` (overflow) via `/oauth/start`. Then check `GET /accounts` (rows 1 and 13).
2. Build step 2 + 3: `/sync`, the stand-in phone script, `/uploads` → `batchCreate`.

## Environment snapshot
- Server: `./bin/server` on `http://localhost:8080` (local, run manually).
- DB: local Postgres 18, database `photobackup`.
- Google Cloud: project `photo-backup-manager`. Test users: `mkesharwani125@gmail.com`, `tanyamishra12346789@gmail.com`, `1by22cs111@bmsit.in`.
