# AGENTS.md

Guide for AI agents (Claude, Codex, etc.) working in this repo. Read this first, then [docs/README.md](docs/README.md).

## What this is
The **backend** for a personal **multi-account Google Photos backup manager**. It decides:
- which new phone photos get uploaded,
- which Google account each one goes to,
- what may be deleted from the phone.

It keeps a registry of what is backed up where. Google Photos stays the storage. An Android app (separate service, built later) will scan the phone and upload bytes.

**Current phase:** backend proof of concept. See [docs/progress/status.md](docs/progress/status.md) for exactly what's done and what's next.

## Stack
Go 1.27 · PostgreSQL 18 (local, no Docker) · Google OAuth 2.0 · Google Photos Library API (REST) · Drive API (quota only).

## Build / run / check
```bash
gofmt -l . && go vet ./... && go build -o bin/server ./cmd/server
./bin/server        # reads ./.env, auto-migrates DB "photobackup", serves http://localhost:8080
TEST_DATABASE_URL='postgres:///photobackup_test?host=/var/run/postgresql' go test -count=1 -p 1 ./...
```
**All tests must pass before committing.** Add tests for every new behaviour, especially the edge cases. Record what is and isn't covered in `docs/progress/status.md`.
If `go` prints nothing (snap wrapper inside a sandbox), use `/snap/go/current/bin/go`. Full details: [docs/setup/local-dev.md](docs/setup/local-dev.md).

## Rules
1. **The repo root is the backend.** Don't create a `backend/` folder or add Android code here.
2. **Commit and push as each piece of work lands.** Plain commit messages, **no `Co-Authored-By` or AI attribution lines**.
3. **Never commit or print secrets.** `.env` (client secret, `TOKEN_ENC_KEY`), `bin/` and `testdata/` are git-ignored. Don't echo tokens or secrets in logs or output.
4. **Keep docs current in the same commit as the work.** At minimum: `docs/progress/status.md` and `docs/progress/worklog.md`. See the rules in [docs/README.md](docs/README.md#rules-for-keeping-docs-current).
5. **Only record a decision in `docs/decisions.md` after the user explicitly confirms it.** Proposals go in `docs/open-questions.md`.
6. **Google API limits matter** (read [docs/architecture/google-apis-feasibility.md](docs/architecture/google-apis-feasibility.md) before designing):
   - The app can only see media **it** uploaded.
   - There is **no delete** API.
   - Albums are flat.
   - The quota is 10k requests per day per project.
7. **Schema changes go in a new numbered file in `internal/store/migrations/`.** Make it idempotent. Never edit an applied migration.
8. **Google sign-in for real accounts is done by the user in their own browser.** Never enter Google passwords.

## Key locations
| Path | What |
|---|---|
| `cmd/server/main.go` | Entry point |
| `internal/api/` | HTTP handlers (`Server.Routes` lists all routes) |
| `internal/google/` | OAuth scopes/config and Google REST calls |
| `internal/store/` | Postgres access + migrations |
| `internal/crypto/` | Token encryption |
| `docs/` | Everything else. Start at `docs/README.md` |
