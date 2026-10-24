package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/Manukesharwani09/photo-backup-manager/internal/planner"
)

// Integration tests. They need a disposable database:
//
//	createdb photobackup_test
//	TEST_DATABASE_URL='postgres:///photobackup_test?host=/var/run/postgresql' go test ./...
//
// Every test wipes all tables, so never point this at the real database.
func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	s, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Pool.Close)
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = s.Pool.Exec(ctx, `TRUNCATE app_user, device, account, account_period, media, device_file,
		album, backup_record, folder_rule RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMigrateIsIdempotent(t *testing.T) {
	s := testStore(t)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("second migrate failed: %v", err)
	}
}

func TestDefaultUserIDIsStable(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	a, err := s.DefaultUserID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.DefaultUserID(ctx)
	if a != b {
		t.Fatalf("ids differ: %d vs %d", a, b)
	}
}

func TestUpsertAccountPeriodsAndReconnect(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid, _ := s.DefaultUserID(ctx)

	a, err := s.UpsertAccount(ctx, uid, "a@gmail.com", "", "primary", []byte("tokA"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.UpsertAccount(ctx, uid, "b@gmail.com", "", "overflow", []byte("tokB"))
	if err != nil {
		t.Fatal(err)
	}

	periods, _ := s.ListPeriods(ctx, uid)
	if len(periods) != 1 || periods[0].AccountID != a || periods[0].EndedAt != nil {
		t.Fatalf("expected one open period for the first account, got %+v", periods)
	}

	// Reconnecting the same account refreshes the token, keeps the id, adds no row or period.
	a2, err := s.UpsertAccount(ctx, uid, "a@gmail.com", "", "primary", []byte("tokA-new"))
	if err != nil {
		t.Fatal(err)
	}
	if a2 != a {
		t.Fatalf("reconnect changed id %d -> %d", a, a2)
	}
	accts, _ := s.ListAccounts(ctx, uid)
	if len(accts) != 2 {
		t.Fatalf("got %d accounts", len(accts))
	}
	if string(accts[0].RefreshTokenEnc) != "tokA-new" {
		t.Fatalf("token not refreshed: %q", accts[0].RefreshTokenEnc)
	}
	if accts[1].ID != b || accts[1].Role != "overflow" {
		t.Fatalf("unexpected second account %+v", accts[1])
	}
	periods, _ = s.ListPeriods(ctx, uid)
	if len(periods) != 1 {
		t.Fatalf("reconnect opened another period: %+v", periods)
	}
}

func TestOnlyOnePrimary(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid, _ := s.DefaultUserID(ctx)
	if _, err := s.UpsertAccount(ctx, uid, "a@gmail.com", "", "primary", []byte("t")); err != nil {
		t.Fatal(err)
	}
	_, err := s.UpsertAccount(ctx, uid, "b@gmail.com", "", "primary", []byte("t"))
	if !errors.Is(err, ErrPrimaryExists) {
		t.Fatalf("want ErrPrimaryExists, got %v", err)
	}
	accts, _ := s.ListAccounts(ctx, uid)
	if len(accts) != 1 {
		t.Fatalf("rejected primary was still saved: %+v", accts)
	}
	// The same account reconnecting as primary is fine.
	if _, err := s.UpsertAccount(ctx, uid, "a@gmail.com", "", "primary", []byte("t2")); err != nil {
		t.Fatalf("reconnecting the primary failed: %v", err)
	}
}

func TestRoleConstraint(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid, _ := s.DefaultUserID(ctx)
	if _, err := s.UpsertAccount(ctx, uid, "a@gmail.com", "", "admin", []byte("t")); err == nil {
		t.Fatal("invalid role accepted")
	}
}

func secondOwner(t *testing.T, s *Store) int64 {
	t.Helper()
	var id int64
	if err := s.Pool.QueryRow(context.Background(), `INSERT INTO app_user (email) VALUES ('other-owner') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// A Google account linked to one owner (as primary or overflow) can't be linked by another owner in any role.
func TestGoogleAccountHasOneOwner(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	me, _ := s.DefaultUserID(ctx)
	other := secondOwner(t, s)

	if _, err := s.UpsertAccount(ctx, me, "Main@Gmail.com", "sub-main", "primary", []byte("t")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertAccount(ctx, me, "extra@gmail.com", "sub-extra", "overflow", []byte("t")); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, email, sub, role string
	}{
		{"my primary as their primary", "main@gmail.com", "sub-main", "primary"},
		{"my primary as their overflow", "main@gmail.com", "sub-main", "overflow"},
		{"my overflow as their primary", "extra@gmail.com", "sub-extra", "primary"},
		{"email differs only in case", "MAIN@gmail.com", "", "primary"},
		{"same Google id, email changed", "renamed@gmail.com", "sub-main", "primary"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := s.UpsertAccount(ctx, other, c.email, c.sub, c.role, []byte("t")); !errors.Is(err, ErrLinkedElsewhere) {
				t.Fatalf("want ErrLinkedElsewhere, got %v", err)
			}
		})
	}
	// Nothing was saved for the other owner, and my accounts are untouched.
	if accts, _ := s.ListAccounts(ctx, other); len(accts) != 0 {
		t.Fatalf("other owner got accounts: %+v", accts)
	}
	mine, _ := s.ListAccounts(ctx, me)
	if len(mine) != 2 || mine[0].Email != "main@gmail.com" || mine[0].Role != "primary" {
		t.Fatalf("my accounts changed: %+v", mine)
	}
	// The other owner can link a Google account nobody has.
	if _, err := s.UpsertAccount(ctx, other, "theirs@gmail.com", "sub-theirs", "primary", []byte("t")); err != nil {
		t.Fatalf("fresh account rejected: %v", err)
	}
	// I can still reconnect my own accounts (token refresh), even with different email case.
	if _, err := s.UpsertAccount(ctx, me, "MAIN@GMAIL.COM", "sub-main", "primary", []byte("t2")); err != nil {
		t.Fatalf("reconnect own primary: %v", err)
	}
}

// The database itself refuses a second primary, even if the Go check were bypassed.
func TestOnePrimaryEnforcedByDatabase(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	me, _ := s.DefaultUserID(ctx)
	if _, err := s.UpsertAccount(ctx, me, "a@gmail.com", "", "primary", []byte("t")); err != nil {
		t.Fatal(err)
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO account (user_id, email, role, refresh_token_enc) VALUES ($1, 'b@gmail.com', 'primary', 'x')`, me)
	if err == nil {
		t.Fatal("database accepted a second primary")
	}
}

// Replacing the rule set soft-deletes the old rules: they stay in the table but are no longer active.
func TestReplaceRules_SoftDeletesOldRules(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid, _ := s.DefaultUserID(ctx)
	first := []planner.Rule{{PathGlob: "Documents", Backup: "include", Cleanup: "protected"}}
	if err := s.ReplaceRules(ctx, uid, first); err != nil {
		t.Fatal(err)
	}
	second := []planner.Rule{{PathGlob: "Private", Backup: "exclude", Cleanup: "allowed"}}
	if err := s.ReplaceRules(ctx, uid, second); err != nil {
		t.Fatal(err)
	}
	active, _ := s.ListRules(ctx, uid)
	if len(active) != 1 || active[0].PathGlob != "Private" {
		t.Fatalf("active rules: %+v", active)
	}
	var total, deleted int
	s.Pool.QueryRow(ctx, `SELECT count(*), count(deleted_at) FROM folder_rule WHERE user_id = $1`, uid).Scan(&total, &deleted)
	if total != 2 || deleted != 1 {
		t.Fatalf("rows=%d soft-deleted=%d, want 2 and 1", total, deleted)
	}
}
