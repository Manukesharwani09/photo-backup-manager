package repository

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/Manukesharwani09/photo-backup-manager/migrations"
)

// migration is one NNN_name.up.sql / NNN_name.down.sql pair.
type migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

func loadMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	byVersion := map[int]*migration{}
	for _, e := range entries {
		name := e.Name()
		dir := ""
		switch {
		case strings.HasSuffix(name, ".up.sql"):
			dir = "up"
		case strings.HasSuffix(name, ".down.sql"):
			dir = "down"
		default:
			continue
		}
		prefix, rest, ok := strings.Cut(name, "_")
		v, err := strconv.Atoi(prefix)
		if !ok || err != nil {
			return nil, fmt.Errorf("migration %s: name must start with a number and _", name)
		}
		body, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		m := byVersion[v]
		if m == nil {
			m = &migration{Version: v, Name: strings.TrimSuffix(strings.TrimSuffix(rest, ".up.sql"), ".down.sql")}
			byVersion[v] = m
		}
		if dir == "up" {
			m.Up = string(body)
		} else {
			m.Down = string(body)
		}
	}
	out := make([]migration, 0, len(byVersion))
	for _, m := range byVersion {
		if m.Up == "" || m.Down == "" {
			return nil, fmt.Errorf("migration %03d_%s needs both .up.sql and .down.sql", m.Version, m.Name)
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func (s *Store) ensureMigrationTable(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INT PRIMARY KEY,
		name       TEXT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`)
	return err
}

// AppliedVersions returns the applied migration versions, ascending.
func (s *Store) AppliedVersions(ctx context.Context) ([]int, error) {
	if err := s.ensureMigrationTable(ctx); err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Migrate applies every pending migration in version order, each in its own transaction.
// Up migrations are idempotent, so a database created before version tracking existed upgrades cleanly.
func (s *Store) Migrate(ctx context.Context) error {
	return s.migrate(ctx, migrations.FS)
}

func (s *Store) migrate(ctx context.Context, fsys fs.FS) error {
	all, err := loadMigrations(fsys)
	if err != nil {
		return err
	}
	applied, err := s.AppliedVersions(ctx)
	if err != nil {
		return err
	}
	done := map[int]bool{}
	for _, v := range applied {
		done[v] = true
	}
	for _, m := range all {
		if done[m.Version] {
			continue
		}
		if err := s.runMigration(ctx, m.Up, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.Version, m.Name); err != nil {
			return fmt.Errorf("migrate up %03d_%s: %w", m.Version, m.Name, err)
		}
	}
	return nil
}

// Rollback reverts the newest `steps` applied migrations, newest first.
func (s *Store) Rollback(ctx context.Context, steps int) error {
	return s.rollback(ctx, migrations.FS, steps)
}

func (s *Store) rollback(ctx context.Context, fsys fs.FS, steps int) error {
	if steps < 1 {
		return fmt.Errorf("rollback steps must be >= 1")
	}
	all, err := loadMigrations(fsys)
	if err != nil {
		return err
	}
	byVersion := map[int]migration{}
	for _, m := range all {
		byVersion[m.Version] = m
	}
	applied, err := s.AppliedVersions(ctx)
	if err != nil {
		return err
	}
	for i := len(applied) - 1; i >= 0 && steps > 0; i, steps = i-1, steps-1 {
		m, ok := byVersion[applied[i]]
		if !ok {
			return fmt.Errorf("applied migration %d has no file to roll back with", applied[i])
		}
		if err := s.runMigration(ctx, m.Down, `DELETE FROM schema_migrations WHERE version = $1 AND $2 <> ''`, m.Version, m.Name); err != nil {
			return fmt.Errorf("rollback %03d_%s: %w", m.Version, m.Name, err)
		}
	}
	return nil
}

func (s *Store) runMigration(ctx context.Context, sql, bookkeeping string, version int, name string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, sql); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, bookkeeping, version, name); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
