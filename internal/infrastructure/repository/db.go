// Package store holds the Postgres access layer.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Manukesharwani09/photo-backup-manager/internal/entity"
)

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

// DefaultUserID returns the single local user, creating it on first run (personal-use phase).
func (s *Store) DefaultUserID(ctx context.Context) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO app_user (email) VALUES ('local')
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id`).Scan(&id)
	return id, err
}

// Account is the linked Google account entity.
type Account = entity.Account

// ErrPrimaryExists is returned when connecting a primary account while a different one is already primary.
var ErrPrimaryExists = errors.New("another account is already primary")

// ErrLinkedElsewhere is returned when the Google account is already linked to a different owner.
// A Google account can belong to one owner only, whatever its role.
var ErrLinkedElsewhere = errors.New("this Google account is already linked to another owner")

// UpsertAccount stores a connected Google account (reconnecting refreshes its token).
// googleSub is Google's stable account id (may be empty in tests); the email is compared case-insensitively.
// The first account of a user opens its active period. Only one account may be primary.
func (s *Store) UpsertAccount(ctx context.Context, userID int64, email, googleSub, role string, tokenEnc []byte) (int64, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var sub *string
	if googleSub != "" {
		sub = &googleSub
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var otherOwner bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM account WHERE user_id <> $1 AND (lower(email) = $2 OR (google_sub IS NOT NULL AND google_sub = $3)))`,
		userID, email, sub).Scan(&otherOwner)
	if err != nil {
		return 0, err
	}
	if otherOwner {
		return 0, ErrLinkedElsewhere
	}

	if role == "primary" {
		var others int
		err := tx.QueryRow(ctx, `
			SELECT count(*) FROM account WHERE user_id = $1 AND role = 'primary' AND lower(email) <> $2`,
			userID, email).Scan(&others)
		if err != nil {
			return 0, err
		}
		if others > 0 {
			return 0, ErrPrimaryExists
		}
	}

	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO account (user_id, email, google_sub, role, refresh_token_enc)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, email) DO UPDATE
		  SET refresh_token_enc = EXCLUDED.refresh_token_enc, role = EXCLUDED.role,
		      google_sub = COALESCE(EXCLUDED.google_sub, account.google_sub)
		RETURNING id`, userID, email, sub, role, tokenEnc).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		// Lost a race with another owner, or a primary was added concurrently.
		if pgErr.ConstraintName == "account_one_primary_per_user" {
			return 0, ErrPrimaryExists
		}
		return 0, ErrLinkedElsewhere
	}
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
