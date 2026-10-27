package account

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/Manukesharwani09/photo-backup-manager/internal/common"
	"github.com/Manukesharwani09/photo-backup-manager/internal/enum"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/client/google"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/crypto"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/httperror"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/logger"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/repository"
)

// SIGN_IN_TTL is how long a started Google sign-in may take to come back.
const SIGN_IN_TTL = 10 * time.Minute

type AccountService interface {
	// StartSignIn returns the Google URL to send the browser to, for linking an account with role.
	StartSignIn(role string) (string, error)
	// CompleteSignIn handles Google's redirect (query has state + code, or error) and links the account.
	CompleteSignIn(ctx context.Context, query url.Values) (ConnectedResponse, error)
	List(ctx context.Context) ([]AccountResponse, error)
	MarkFull(ctx context.Context, accountID int64) error
	// MarkAvailable clears the full flag after the user freed space in that account.
	MarkAvailable(ctx context.Context, accountID int64) error
	// MakeCurrent makes the account the one receiving new backups (closes the current period as "manual").
	MakeCurrent(ctx context.Context, accountID int64) error
}

type accountService struct {
	repo     *repository.Store
	oauth    *oauth2.Config
	access   *Access
	tokenKey []byte
	userID   int64

	mu     sync.Mutex
	states map[string]pendingSignIn // OAuth state -> what the sign-in is for
}

type pendingSignIn struct {
	role    enum.AccountRole
	expires time.Time
}

func NewAccountService(repo *repository.Store, oauth *oauth2.Config, access *Access, tokenKey []byte, userID int64) AccountService {
	return &accountService{repo: repo, oauth: oauth, access: access, tokenKey: tokenKey, userID: userID, states: map[string]pendingSignIn{}}
}

func (s *accountService) StartSignIn(role string) (string, error) {
	r := enum.AccountRole(role)
	if role == "" {
		r = enum.ACCOUNT_ROLE_OVERFLOW
	}
	if !r.IsValid() {
		return "", httperror.NewHTTPError(httperror.BAD_REQUEST, "role must be primary or overflow")
	}
	b := make([]byte, 16)
	rand.Read(b)
	state := hex.EncodeToString(b)
	s.mu.Lock()
	for k, p := range s.states { // drop abandoned sign-ins
		if time.Now().After(p.expires) {
			delete(s.states, k)
		}
	}
	s.states[state] = pendingSignIn{role: r, expires: time.Now().Add(SIGN_IN_TTL)}
	s.mu.Unlock()
	return google.AuthURL(s.oauth, state), nil
}

func (s *accountService) CompleteSignIn(ctx context.Context, q url.Values) (ConnectedResponse, error) {
	log := logger.FromContext(ctx)
	if e := q.Get("error"); e != "" {
		return ConnectedResponse{}, httperror.NewHTTPError(httperror.BAD_REQUEST, "Google returned: "+e)
	}
	s.mu.Lock()
	pending, ok := s.states[q.Get("state")]
	delete(s.states, q.Get("state")) // single use, even if it fails below
	s.mu.Unlock()
	if !ok || time.Now().After(pending.expires) {
		return ConnectedResponse{}, httperror.NewHTTPError(httperror.BAD_REQUEST, "unknown or expired sign-in, start again at /oauth/start")
	}

	tok, err := s.oauth.Exchange(ctx, q.Get("code"))
	if err != nil {
		log.Error("oauth code exchange failed", "error", err)
		return ConnectedResponse{}, httperror.NewHTTPError(httperror.BAD_GATEWAY, "Google rejected the sign-in code; start again at /oauth/start")
	}
	if tok.RefreshToken == "" {
		return ConnectedResponse{}, httperror.NewHTTPError(httperror.BAD_GATEWAY, "Google did not return a refresh token; remove the app's access at myaccount.google.com/permissions and retry")
	}
	granted, _ := tok.Extra("scope").(string)
	if missing := google.MissingScopes(granted); len(missing) > 0 {
		return ConnectedResponse{}, httperror.NewHTTPError(httperror.BAD_REQUEST,
			"Some permissions were not granted (all checkboxes must be ticked). Missing: "+strings.Join(missing, ", ")+". Start again at /oauth/start.")
	}
	email, sub, err := google.Identity(ctx, s.oauth.Client(ctx, tok))
	if err != nil {
		log.Error("reading account identity failed", "error", err)
		return ConnectedResponse{}, httperror.NewHTTPError(httperror.BAD_GATEWAY, "could not read the Google account's identity")
	}
	enc, err := crypto.Seal(s.tokenKey, []byte(tok.RefreshToken))
	if err != nil {
		return ConnectedResponse{}, err
	}
	id, err := s.repo.UpsertAccount(ctx, s.userID, email, sub, string(pending.role), enc)
	switch {
	case errors.Is(err, repository.ErrPrimaryExists):
		return ConnectedResponse{}, httperror.NewHTTPError(httperror.CONFLICT, "Another account is already primary. Connect "+email+" with role=overflow instead.")
	case errors.Is(err, repository.ErrLinkedElsewhere):
		return ConnectedResponse{}, httperror.NewHTTPError(httperror.CONFLICT, email+" is already linked to another owner and can't be linked again.")
	case err != nil:
		return ConnectedResponse{}, err
	}
	log.Info("google account connected", "account_id", id, "email", logger.MaskEmail(email), "role", pending.role)
	return ConnectedResponse{AccountID: id, Email: email, Role: string(pending.role)}, nil
}

func (s *accountService) List(ctx context.Context) ([]AccountResponse, error) {
	accts, err := s.repo.ListAccounts(ctx, s.userID)
	if err != nil {
		return nil, err
	}
	periods, err := s.repo.ListPeriods(ctx, s.userID)
	if err != nil {
		return nil, err
	}
	out := make([]AccountResponse, 0, len(accts))
	for _, a := range accts {
		v := AccountResponse{ID: a.ID, Email: a.Email, Role: string(a.Role), Active: a.Active, Full: a.FullFlag, ActivePeriods: PeriodsOf(periods, a.ID)}
		if q, err := s.access.Quota(ctx, a); err != nil {
			v.StorageError = err.Error()
		} else {
			v.StorageLimit, v.StorageUsed, v.StorageFree = q.Limit, q.Usage, q.Remaining()
			v.StorageFreeGB = common.FormatGB(q.Remaining())
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *accountService) MarkFull(ctx context.Context, accountID int64) error {
	err := s.repo.MarkFull(ctx, s.userID, accountID)
	if errors.Is(err, repository.ErrNotFound) {
		return httperror.NewHTTPError(httperror.NOT_FOUND, "unknown account")
	}
	if err == nil {
		logger.FromContext(ctx).Info("account marked full", "account_id", accountID)
	}
	return err
}

func (s *accountService) MarkAvailable(ctx context.Context, accountID int64) error {
	err := s.repo.MarkAvailable(ctx, s.userID, accountID)
	if errors.Is(err, repository.ErrNotFound) {
		return httperror.NewHTTPError(httperror.NOT_FOUND, "unknown account")
	}
	if err == nil {
		logger.FromContext(ctx).Info("account marked available", "account_id", accountID)
	}
	return err
}

func (s *accountService) MakeCurrent(ctx context.Context, accountID int64) error {
	acct, err := s.repo.AccountOwned(ctx, s.userID, accountID)
	if errors.Is(err, repository.ErrNotFound) {
		return httperror.NewHTTPError(httperror.NOT_FOUND, "unknown account")
	}
	if err != nil {
		return err
	}
	switch {
	case !acct.Active:
		return httperror.NewHTTPError(httperror.CONFLICT, "account is disabled")
	case acct.FullFlag:
		return httperror.NewHTTPError(httperror.CONFLICT, "account is full; mark it available first")
	}
	accts, err := s.repo.PlannerAccounts(ctx, s.userID)
	if err != nil {
		return err
	}
	for _, a := range accts {
		if a.ID == accountID && a.IsCurrent {
			return nil // already current: nothing to record
		}
	}
	if err := s.repo.SwitchCurrent(ctx, s.userID, accountID, string(enum.PERIOD_END_MANUAL)); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("current account switched", "account_id", accountID, "reason", enum.PERIOD_END_MANUAL)
	return nil
}

// PeriodsOf picks one account's active periods out of the owner's list.
func PeriodsOf(all []repository.Period, accountID int64) []Period {
	out := []Period{}
	for _, p := range all {
		if p.AccountID == accountID {
			out = append(out, Period{StartedAt: p.StartedAt, EndedAt: p.EndedAt, EndReason: p.EndReason})
		}
	}
	return out
}
