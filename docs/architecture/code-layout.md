# Code layout & conventions

The repo root **is the backend** (Go + PostgreSQL). The Android app will be a separate service/repo later; don't add it here.

The structure follows the team's **Horizon** layout (DDD-inspired: application / entity / infrastructure) and its coding practices, adapted to a small service. It uses the Go standard library `net/http` and `pgx` rather than Gin and GORM.

## Layout
```
cmd/
  main.go                 Entry point with sub-commands: appserver | migrate | rollback N
  phonesim/main.go        Stand-in for the Android app: scans a folder, syncs, uploads bytes, restores
migrations/               Versioned SQL: NNN_name.up.sql + NNN_name.down.sql, embedded (embed.go).
                          Applied versions are tracked in schema_migrations.
scripts/gen_testdata.py   Generates phone-like test photos (with GPS) into testdata/
internal/
  application/            Business features. One package per feature:
    service.go              Services struct + NewServices(): the single place services are wired
    account/                Google sign-in, accounts, mark full. access.go = Google client/token for an account
    backup/                 devices, sync (upload plan), upload completion. album.go = EnsureAlbum
    rule/                   folder rules (replace = soft delete old)
    cleanup/                which phone files are safe to delete
    restore/                download links back into original folders
    similar/                look-alike groups + "🗑 To delete" album
    cloudcheck/             find photos deleted in Google Photos (+ scheduler)
    dashboard/              per-account summary, alerts, photos-by-date
      each feature has:     controller.go (HTTP only) · service.go (interface + logic) · dto.go (request/response)
  entity/                 One struct per table (data only)
  enum/                   Typed string enums with IsValid(): roles, file states, backup statuses, rule modes…
  planner/                Pure decision logic, no I/O: rule matching, account choice, cleanup eligibility,
                          similar grouping. Most edge cases are unit-tested here.
  common/                 Controller helpers: DecodeJSON, WriteJSON, FormatGB, PathInt64
  config/                 Env vars with .env fallback (never overrides real env vars)
  infrastructure/
    appserver/            routes.go (NewRouter: the full API list) · server.go (error mapping, correlation id,
                          request log). *_test.go = end-to-end API tests against a fake Google + the test DB
    repository/           Postgres access (pgx/v5): db.go (Store, accounts), one file per area, migrate.go
    client/google/        OAuth config/scopes + thin REST calls (Photos Library, Drive quota, userinfo)
    crypto/               AES-256-GCM for refresh tokens at rest
    httperror/            HTTPError + codes (BAD_REQUEST, NOT_FOUND, CONFLICT, …)
    logger/               slog logger, per-request logger with correlation id, MaskEmail
docs/                     Start at docs/README.md
```

## Request flow
```
HTTP → appserver.NewRouter → handle(controller method)
         controller: decode request (common.DecodeJSON) → call service → common.WriteJSON
         service:    validation + business rules (planner) → repository / client.google
         errors:     services return httperror.NewHTTPError(CODE, msg) for client errors;
                     anything else becomes a 500 with details only in the log (by correlation id)
```

## Conventions
- **Layering:** controllers are thin. Logic lives in services; SQL lives only in `repository`. Add a feature by:
  1. creating `application/<feature>/{controller,service,dto}.go`;
  2. adding it to `application.Services` and `NewServices`;
  3. registering its routes in `appserver/routes.go`;
  4. listing the routes in [overview.md](overview.md#backend-api).
- **API:** everything under `/v1/…`. Only `/oauth/start` and `/oauth/callback` are unversioned (the callback is registered with Google). JSON fields are `snake_case`; unknown request fields are rejected.
- **Naming:**
  - constants and enum values: `UPPER_SNAKE_CASE` (`enum.FILE_STATE_PENDING`, `httperror.CONFLICT`);
  - types: `PascalCase` with a role suffix (`BackupService`, `SyncRequest`);
  - tests: `Test<Thing>_<Scenario>` for new tests.
- **Enums over magic strings:** use `internal/enum`. The DB stores the same string values.
- **Errors:** never leak raw errors to clients. Use `httperror` codes `400` (bad input), `404` (not found), `409` (conflict), `502` (Google failed); everything else is `500`.
- **Logging:** structured `slog` through `logger.FromContext(ctx)` (it carries the correlation id). **Mask emails** with `logger.MaskEmail`; never log tokens, codes or query strings.
- **Database:**
  - every table has `created_at`/`updated_at` (a trigger keeps `updated_at` current);
  - replace-style updates soft-delete (`folder_rule.deleted_at`);
  - schema changes are a **new** numbered up/down migration pair; never edit an applied one;
  - `make rollback n=1` reverts the newest.
- **Dependencies:** standard library first. Currently only `jackc/pgx/v5` and `golang.org/x/oauth2`. Google APIs are called with plain REST.
- **Secrets:** only in `.env` (git-ignored, mode 600). Refresh tokens are encrypted with `TOKEN_ENC_KEY` before they go into the DB.
- **Single owner for now:** `repository.DefaultUserID`. App ↔ backend sign-in comes later (see [data-model.md](data-model.md#how-accounts-are-managed)).
- **Before committing:** `make check` (gofmt + vet + all tests).

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
