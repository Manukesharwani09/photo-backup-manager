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
make run            # go run ./cmd appserver: reads ./.env, migrates DB "photobackup", serves http://localhost:8080
make check          # gofmt + vet + all tests (DB tests use photobackup_test)
make migrate | make rollback n=1 | make build | make phonesim   (make help lists all)
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
7. **Schema changes are a new numbered pair `migrations/NNN_name.up.sql` + `.down.sql`.** Make the up idempotent. Never edit an applied migration.
8. **Follow the layering and conventions in [docs/architecture/code-layout.md](docs/architecture/code-layout.md)** (Horizon-style):
   - controller → service → repository;
   - `httperror` for client errors;
   - typed enums;
   - `/v1` routes;
   - masked emails in logs.
9. **Google sign-in for real accounts is done by the user in their own browser.** Never enter Google passwords.

## Key locations
| Path | What |
|---|---|
| `cmd/main.go` | Entry point (`appserver`, `migrate`, `rollback N`) |
| `internal/application/<feature>/` | Business features: `controller.go`, `service.go`, `dto.go`; wired in `application/service.go` |
| `internal/infrastructure/appserver/routes.go` | The full API (`NewRouter`); end-to-end tests live next to it |
| `internal/entity/`, `internal/enum/` | Table structs, typed enums |
| `internal/planner/` | Pure decision rules (most edge-case unit tests) |
| `internal/infrastructure/repository/` | All SQL |
| `internal/infrastructure/client/google/` | Google OAuth + REST calls |
| `migrations/` | Versioned up/down SQL |
| `docs/` | Everything else. Start at `docs/README.md` |
