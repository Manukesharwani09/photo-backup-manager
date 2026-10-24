# Work log

One entry per task (not per commit), newest first. Each entry says what was done, which commits it covers, and anything a future agent must know.

## 2026-10-24: Account ownership + data-model doc
- Commit `1b53e1b`. Migration 004:
  - `account.google_sub`;
  - unique `google_sub` and `lower(email)` across all owners;
  - one primary per owner, as a partial unique index.
- `UpsertAccount` returns `ErrLinkedElsewhere`, which the API turns into a 409. `google.Identity` now returns email + sub.
- Tests:
  - 5 ways another owner might try to link an existing account (primary/overflow, swapped roles, different email case, renamed email with the same sub);
  - the DB refuses a second primary even when the Go check is bypassed.
- Live DB migrated. The two live accounts get their `google_sub` filled in on their next reconnect.
- Added `docs/architecture/data-model.md` (ER diagram, tables, which Go structs read which tables, account rules).
- A4 attempt via photos.google.com: search never finished loading in a CDP-driven tab, so it stays a manual check. A5 has no samples. A6 isn't worth forcing.

## 2026-10-24: Cleanup, restore, similar, To delete, cloud check, dashboard (proof-of-concept steps 9–12, 15)
- Commits: `97c7b3e` (cleanup), `70dbe5f` (restore), `931ac01` (similar + To delete), `7c36380` (cloud check + scheduler), `48d57c0` (dashboard + photos/where), `a646ebf` (count fix).
- Live against real Google:
  - The cleanup preview was correct.
  - Restore to a second simulated phone worked. **A3: Google strips GPS and changes bytes on download.**
  - The "🗑 To delete" album was created and an item added.
  - The cloud check ran 5/5.
  - The dashboard and photos/where returned correct results.
- Bugs found by tests and live runs, then fixed:
  1. A deliberately deleted duplicate (via To delete) went back to pending on the next sync. Sync now checks `DeletedOnPurpose`.
  2. The dashboard double-counted restored copies (they link to the same Google item). It now counts distinct Google items.
- Corrections to earlier design statements:
  - Albums can't be deleted via the API, so the empty To delete album stays and is reused.
  - `RecordRestored` originally swallowed every DB error; now only "no rows" means "not backed up".
- Gotchas:
  - Postgres TEXT can't hold NUL bytes. The To delete album key is `:to-delete:`.
  - Test fakes need `Content-Type: application/json`.
  - `DriveAPI` and `PhotosAPI` are variables so tests can redirect them.

## 2026-10-24: Sync + upload (proof-of-concept steps 2–8)
- Commits: `9418138` (planner), `4c671e5` (Photos client), `09b7491` (sync/upload API + end-to-end tests), `954e698` (phonesim + test photo generator).
- What was built:
  - `internal/plan`: pure decisions (rule matching, album title, account choice).
  - `internal/google/photos.go`: upload, albums, batchCreate, batchGet.
  - `internal/store/backup.go` and migration 002: a unique index allows only one live backup per photo.
  - `internal/api/sync.go`: `/devices`, `/sync`, `/uploads`, `/rules`, `/accounts/{id}/full`.
  - `cmd/phonesim`: the stand-in phone.
- Live run against real Google:
  - The first scan skipped 2 start-point photos.
  - 3 new photos were uploaded to `mkesharwani125` and verified, in albums `DCIM › Camera` and `Documents › Bills`.
  - The duplicate copy and the excluded `Private` photo were not uploaded.
  - The re-sync planned nothing.
  - **A2 is verified**, so the split upload design is confirmed.
- Gotchas:
  - The `store` and `api` tests share `photobackup_test`, so run `go test -p 1`.
  - A fake OAuth token endpoint must reply with `Content-Type: application/json`, or `x/oauth2` can't find the token.
  - PIL needs EXIF GPS as float rationals, not `(num, den)` tuples.
- Account switch: tested with a fake Google only, so the real primary isn't left flagged full.

## 2026-10-24: Real sign-in verified
- Connected `mkesharwani125@gmail.com` (primary) and `tanyamishra12346789@gmail.com` (overflow), at the user's request, by driving Brave over CDP. No passwords were used; both accounts were already signed in.
- Sign-in flow seen:
  1. account chooser
  2. "Google hasn't verified this app" → Continue
  3. "Sign in to Photo Backup Manager" (email) → Continue
  4. permissions screen with a "Select all" checkbox plus 4 permissions → Continue
  5. callback
- `/accounts` returned live quotas (12.08 GB and 14.96 GB free of 15 GB), and the primary account has an open active period.
- A1 and A7 are verified.

## 2026-10-24: App design brief
- Added `docs/app/design-brief.md` so a separate agent can start the Android app's UX/UI design while the backend proof of concept continues.
- It covers: screens, flows, vocabulary, edge-case states, platform constraints, provisional backend data, and open design questions.
- Navigation and styling are marked as proposals (not confirmed).

## 2026-10-24: Tests + sign-in edge-case fixes
- Added unit tests (`crypto`, `config`, `google`, `api`) and DB integration tests (`store`, against a disposable `photobackup_test` DB). 26 tests, all passing.
- Fixed 2 gaps found while reviewing:
  1. **Unticked permissions.** Sign-in is now rejected with a clear message when the user unticks a permission on Google's consent screen. Previously the account would be saved and fail later.
  2. **Two primaries.** Only one account can be `primary`. A second primary gets a 409, and you reconnect it as overflow.
- Also: abandoned sign-in states are cleaned up on the next `/oauth/start`.

## 2026-10-24: Docs reorganised for agents
- Docs moved into `product/`, `architecture/`, `setup/`, `progress/`, plus `decisions.md` and `open-questions.md`. Added `AGENTS.md` (agent entry point) and `CLAUDE.md` (imports AGENTS.md).
- The old `04-open-questions.md` was split into `decisions.md`, `open-questions.md` and `progress/status.md`.

## 2026-10-24: Backend skeleton (Go)
- Commits: `3d8818a` (skeleton), plus the follow-up move to the repo root.
- Built:
  - multi-account Google OAuth (`/oauth/start`, `/oauth/callback`)
  - refresh tokens AES-GCM encrypted in Postgres
  - all 9 tables via an embedded migration
  - `GET /accounts` with live remaining storage (Drive `about.get`)
- Gotchas:
  - The first version was put in `backend/`. The user wants the repo root to be the backend, so it was moved.
  - The snap `go` wrapper prints nothing in sandboxed shells. Use `/snap/go/current/bin/go`.
  - `pkill -f 'bin/server'` from a shell also matches that shell's own command line. Kill the server by PID instead.
- Not yet verified: sign-in with real accounts (waiting on the user).

## 2026-10-24: Google Cloud setup
- Commit: `1ae51e4`.
- Project `photo-backup-manager` (owner `mkesharwani125@gmail.com`):
  - Photos Library + Drive APIs enabled
  - consent screen External/Testing (User Data Policy agreed by the user)
  - 3 test users and 4 scopes
  - Web OAuth client `photo-backup-backend-local`
- Done by driving the user's Brave over the Chrome DevTools Protocol (`brave --remote-debugging-port=9222`). Console screens can't be scripted any other way.
- The client secret was written straight to `.env`. The Console can't show it again; if it's lost, create a new secret.
- An earlier attempt under `1by22cs111@bmsit.in` (project `capable-avatar-512606-a8`) was abandoned.

## 2026-10-22 → 24: Design & feasibility
- Commits: `f204475`, `442b369`, `fc28ac3`, `08b6248`, `a9da539`, `203f14e`.
- Problem statement, feasibility study (Google Photos API after the 31 Mar 2025 scope removal), architecture, and the Drive-vs-Photos discussion.
- Every decision is in [../decisions.md](../decisions.md).
