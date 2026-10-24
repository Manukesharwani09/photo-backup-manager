// Package store holds the Postgres access layer.
package store

import (
	"context"
	"embed"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	Pool *pgxpool.Pool
}

func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Store{Pool: pool}, nil
}

// Migrate applies every embedded migration. Migrations are written to be idempotent.
func (s *Store) Migrate(ctx context.Context) error {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		sql, err := migrations.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		if _, err := s.Pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("migration %s: %w", e.Name(), err)
		}
	}
	return nil
}

// DefaultUserID returns the single local user, creating it on first run (personal-use phase).
func (s *Store) DefaultUserID(ctx context.Context) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO app_user (email) VALUES ('local')
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id`).Scan(&id)
	return id, err
}

type Account struct {
	ID              int64
	Email           string
	Role            string
	Active          bool
	FullFlag        bool
	RefreshTokenEnc []byte
}

// UpsertAccount stores a connected Google account. The first account of a user opens its active period.
func (s *Store) UpsertAccount(ctx context.Context, userID int64, email, role string, tokenEnc []byte) (int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO account (user_id, email, role, refresh_token_enc)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, email) DO UPDATE
		  SET refresh_token_enc = EXCLUDED.refresh_token_enc, role = EXCLUDED.role
		RETURNING id`, userID, email, role, tokenEnc).Scan(&id)
	if err != nil {
		return 0, err
	}

	var openPeriods int
	err = tx.QueryRow(ctx, `
		SELECT count(*) FROM account_period p JOIN account a ON a.id = p.account_id
		WHERE a.user_id = $1 AND p.ended_at IS NULL`, userID).Scan(&openPeriods)
	if err != nil {
		return 0, err
	}
	if openPeriods == 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO account_period (account_id) VALUES ($1)`, id); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit(ctx)
}

func (s *Store) ListAccounts(ctx context.Context, userID int64) ([]Account, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, email, role, active, full_flag, refresh_token_enc
		FROM account WHERE user_id = $1 ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Email, &a.Role, &a.Active, &a.FullFlag, &a.RefreshTokenEnc); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

type Period struct {
	AccountID int64
	StartedAt string
	EndedAt   *string
	EndReason *string
}

func (s *Store) ListPeriods(ctx context.Context, userID int64) ([]Period, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT p.account_id, p.started_at::text, p.ended_at::text, p.end_reason
		FROM account_period p JOIN account a ON a.id = p.account_id
		WHERE a.user_id = $1 ORDER BY p.started_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Period
	for rows.Next() {
		var p Period
		if err := rows.Scan(&p.AccountID, &p.StartedAt, &p.EndedAt, &p.EndReason); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
