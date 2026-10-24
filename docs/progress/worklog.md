# Work log

One entry per task (not per commit), newest first. Each entry says what was done, which commits it covers, and anything a future agent must know.

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
