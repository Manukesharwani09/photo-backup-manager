# Local development

## Prerequisites (on the dev machine)
- **Go 1.27.** Installed as a snap. In a normal terminal, `go` works. In sandboxed or agent shells, the snap wrapper can print nothing; call the binary directly with `/snap/go/current/bin/go`, or `export PATH=/snap/go/current/bin:$PATH`.
- **PostgreSQL 18**, running locally on the Unix socket `/var/run/postgresql`. The OS user `manu-kesharwani` can create databases. No Docker is used.
- **A Google Cloud project with an OAuth client.** Already done; see [google-cloud.md](google-cloud.md).

## One-time setup
```bash
createdb photobackup        # already created
# .env in the repo root (git-ignored) must contain:
#   GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GOOGLE_REDIRECT_URL, TOKEN_ENC_KEY
# TOKEN_ENC_KEY was generated with: head -c 32 /dev/urandom | base64
```
See [../architecture/code-layout.md](../architecture/code-layout.md#environment-variables) for all variables.

## Build & run
```bash
gofmt -l . && go vet ./... && go build -o bin/server ./cmd/server
./bin/server                 # reads ./.env, migrates the DB, listens on http://localhost:8080
```
Migrations run automatically on every start.

## Tests
```bash
createdb photobackup_test   # once; disposable, every DB test wipes it
TEST_DATABASE_URL='postgres:///photobackup_test?host=/var/run/postgresql' go test -count=1 -p 1 ./...
```
`-p 1` matters: the `store` and `api` packages share the test DB, so they must not run in parallel. Without `TEST_DATABASE_URL`, the DB tests in `internal/store` and `internal/api` are skipped, and the rest still run. **Never point `TEST_DATABASE_URL` at `photobackup`.**

## Connect Google accounts
Open these in a browser where the account is signed in:
- `http://localhost:8080/oauth/start?role=primary`
- `http://localhost:8080/oauth/start?role=overflow`

Notes:
- **"Google hasn't verified this app":** expected while the app is in Testing. Click Advanced → Go to Photo Backup Manager.
- **Permissions screen:** tick **all** checkboxes.
- **Only test users can sign in.** They're listed in [google-cloud.md](google-cloud.md).
- **Refresh tokens expire after 7 days in Testing mode.** Reconnect the same way.

Check the result: `curl -s http://localhost:8080/accounts`

## Inspect the DB
```bash
psql -d photobackup -c '\dt'
psql -d photobackup -c 'select id, email, role, active, full_flag from account'
```
