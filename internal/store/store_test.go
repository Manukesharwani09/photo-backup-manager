package store

import (
	"context"
	"errors"
	"os"
	"testing"
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

	a, err := s.UpsertAccount(ctx, uid, "a@gmail.com", "primary", []byte("tokA"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.UpsertAccount(ctx, uid, "b@gmail.com", "overflow", []byte("tokB"))
	if err != nil {
		t.Fatal(err)
	}

	periods, _ := s.ListPeriods(ctx, uid)
	if len(periods) != 1 || periods[0].AccountID != a || periods[0].EndedAt != nil {
		t.Fatalf("expected one open period for the first account, got %+v", periods)
	}

	// Reconnecting the same account refreshes the token, keeps the id, adds no row or period.
	a2, err := s.UpsertAccount(ctx, uid, "a@gmail.com", "primary", []byte("tokA-new"))
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
	if _, err := s.UpsertAccount(ctx, uid, "a@gmail.com", "primary", []byte("t")); err != nil {
		t.Fatal(err)
	}
	_, err := s.UpsertAccount(ctx, uid, "b@gmail.com", "primary", []byte("t"))
	if !errors.Is(err, ErrPrimaryExists) {
		t.Fatalf("want ErrPrimaryExists, got %v", err)
	}
	accts, _ := s.ListAccounts(ctx, uid)
	if len(accts) != 1 {
		t.Fatalf("rejected primary was still saved: %+v", accts)
	}
	// The same account reconnecting as primary is fine.
	if _, err := s.UpsertAccount(ctx, uid, "a@gmail.com", "primary", []byte("t2")); err != nil {
		t.Fatalf("reconnecting the primary failed: %v", err)
	}
}

func TestRoleConstraint(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	uid, _ := s.DefaultUserID(ctx)
	if _, err := s.UpsertAccount(ctx, uid, "a@gmail.com", "admin", []byte("t")); err == nil {
		t.Fatal("invalid role accepted")
	}
}
