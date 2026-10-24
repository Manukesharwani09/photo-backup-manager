# Code layout & conventions

The repo root **is the backend** (Go + PostgreSQL). The Android app will be a separate service/repo later; don't add it here.

## Layout
```
cmd/server/main.go        Entry point: load config → open DB → run migrations → start cloud-check scheduler → serve HTTP
cmd/phonesim/main.go      Stand-in for the Android app: scans a folder, syncs, uploads bytes, restores
scripts/gen_testdata.py   Generates phone-like test photos (with GPS) into testdata/
internal/
  api/                    HTTP handlers, one file per area: accounts.go (sign-in, /accounts, Routes),
                          sync.go (devices, sync, uploads, rules), cleanup.go, restore.go, similar.go,
                          cloudcheck.go, dashboard.go. Server.Routes() is the single place routes are registered.
                          *_test.go here are end-to-end tests against a fake Google + the test DB.
  plan/                   Pure decision logic, no I/O: rule matching, album titles, account choice,
                          cleanup eligibility, similar-photo grouping. Most edge cases are unit-tested here.
  config/                 Reads env vars, falling back to .env (never overrides real env vars).
  crypto/                 AES-256-GCM Seal/Open for secrets at rest (Google refresh tokens).
  google/                 Google OAuth config + scopes, and thin REST calls (userinfo, Drive quota;
                          Photos calls go here too).
  store/                  Postgres access (pgx/v5 pool) + embedded migrations.
    migrations/NNN_*.sql  Applied in name order on every start. Must be idempotent
                          (CREATE ... IF NOT EXISTS). Add a new numbered file; don't edit applied ones.
docs/                     Product, architecture, setup, decisions, progress. Start at docs/README.md.
```

## Conventions
- **Go version:** 1.27 (`go.mod`). Module path: `github.com/Manukesharwani09/photo-backup-manager`.
- **Dependencies:** standard library first. Currently only `jackc/pgx/v5` (Postgres) and `golang.org/x/oauth2`.
- **Google APIs:** called with plain REST + an oauth2 HTTP client, not the large generated Google client libraries. The Photos Library API has no maintained Go client.
- **Secrets:** only in `.env` (git-ignored, mode 600). Refresh tokens are encrypted with `TOKEN_ENC_KEY` before they go into the DB. Never log or print tokens or secrets.
- **Single user for now:** `store.DefaultUserID` creates one local `app_user`. Multi-user auth comes later.
- **Errors:** handlers log and return `what: err` with a 500 via `httpError`. Bad input gets a 400 with a plain message.
- **Format and check before committing:** `gofmt -l .` (must print nothing), `go vet ./...`, `go build ./...`.

## Environment variables
| Var | Required | Default | Purpose |
|---|---|---|---|
| `GOOGLE_CLIENT_ID` | yes | — | OAuth client (see [setup/google-cloud.md](../setup/google-cloud.md)) |
| `GOOGLE_CLIENT_SECRET` | yes | — | OAuth client secret |
| `GOOGLE_REDIRECT_URL` | no | `http://localhost:8080/oauth/callback` | Must match the client's redirect URI exactly |
| `TOKEN_ENC_KEY` | yes | — | 32 random bytes, base64. **Changing it makes stored tokens unreadable**, so accounts would need to reconnect |
| `DATABASE_URL` | no | `postgres:///photobackup?host=/var/run/postgresql` | Local Postgres via Unix socket |
| `LISTEN_ADDR` | no | `localhost:8080` | HTTP listen address |
| `CLOUD_CHECK_INTERVAL` | no | `24h` | How often to look for photos deleted in Google Photos (Go duration; `0` disables) |
