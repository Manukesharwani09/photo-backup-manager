package repository

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/Manukesharwani09/photo-backup-manager/migrations"
)

func TestMigrate_RollbackAndReapply(t *testing.T) {
	s := testStore(t) // fully migrated
	ctx := context.Background()
	all, err := loadMigrations(migrationsFS())
	if err != nil {
		t.Fatal(err)
	}
	applied, _ := s.AppliedVersions(ctx)
	if len(applied) != len(all) {
		t.Fatalf("applied %v, want %d migrations", applied, len(all))
	}
	// Roll back the newest migration (005 audit columns): updated_at disappears.
	if err := s.Rollback(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if hasColumn(t, s, "account", "updated_at") {
		t.Fatal("updated_at still present after rollback")
	}
	if a, _ := s.AppliedVersions(ctx); len(a) != len(all)-1 {
		t.Fatalf("applied after rollback: %v", a)
	}
	// Migrating again restores it, and a second Migrate is a no-op.
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if !hasColumn(t, s, "account", "updated_at") {
		t.Fatal("updated_at missing after re-migrate")
	}
	// The trigger keeps updated_at current.
	uid, _ := s.DefaultUserID(ctx)
	var before, after string
	s.Pool.QueryRow(ctx, `SELECT updated_at::text FROM app_user WHERE id = $1`, uid).Scan(&before)
	s.Pool.Exec(ctx, `UPDATE app_user SET email = email WHERE id = $1`, uid)
	s.Pool.QueryRow(ctx, `SELECT updated_at::text FROM app_user WHERE id = $1`, uid).Scan(&after)
	if before == after {
		t.Fatal("updated_at not bumped by trigger")
	}
}

func TestMigrate_RollbackEverythingThenUp(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if err := s.Rollback(ctx, 100); err != nil {
		t.Fatal(err)
	}
	if a, _ := s.AppliedVersions(ctx); len(a) != 0 {
		t.Fatalf("still applied: %v", a)
	}
	if hasTable(t, s, "account") {
		t.Fatal("tables remain after full rollback")
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if !hasTable(t, s, "account") {
		t.Fatal("tables not recreated")
	}
}

func TestLoadMigrations_Validation(t *testing.T) {
	cases := map[string]fstest.MapFS{
		"missing down": {"001_a.up.sql": {Data: []byte("SELECT 1")}},
		"bad prefix":   {"x_a.up.sql": {Data: []byte("SELECT 1")}, "x_a.down.sql": {Data: []byte("SELECT 1")}},
	}
	for name, fsys := range cases {
		if _, err := loadMigrations(fsys); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	ok := fstest.MapFS{
		"002_b.up.sql": {Data: []byte("u2")}, "002_b.down.sql": {Data: []byte("d2")},
		"001_a.up.sql": {Data: []byte("u1")}, "001_a.down.sql": {Data: []byte("d1")},
		"README.md": {Data: []byte("ignored")},
	}
	ms, err := loadMigrations(ok)
	if err != nil || len(ms) != 2 || ms[0].Version != 1 || ms[1].Name != "b" || ms[1].Down != "d2" {
		t.Fatalf("got %+v %v", ms, err)
	}
	if err := (&Store{}).rollback(context.Background(), ok, 0); err == nil {
		t.Fatal("rollback 0 steps accepted")
	}
}

func hasColumn(t *testing.T, s *Store, table, col string) bool {
	var ok bool
	s.Pool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`, table, col).Scan(&ok)
	return ok
}

func hasTable(t *testing.T, s *Store, table string) bool {
	var ok bool
	s.Pool.QueryRow(context.Background(), `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&ok)
	return ok
}

func migrationsFS() fs.FS { return migrations.FS }
