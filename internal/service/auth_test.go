package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"eventAI/internal/entities/core"
	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/repo"
)

func TestAuthServiceLoginWithMAXCreatesSession(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_900_000_100, 0).UTC()
	authRepo := &stubAuthRepository{
		authResult: core.User{
			ID:       "user-1",
			FullName: "Иван Иванов",
			Phone:    strPtr("79141010000"),
		},
	}
	maxProvider := &stubMAXProvider{
		identity: MAXIdentity{
			ProviderUserID: "max-123",
			FullName:       "  Иван Иванов  ",
			Phone:          strPtr("+79141010000"),
			AuthDate:       now,
		},
	}
	tokenIssuer := &stubTokenIssuer{
		token:     "jwt-token",
		expiresAt: time.Unix(1_900_000_000, 0).UTC().Unix(),
	}

	svc := NewAuthService(authRepo, maxProvider, tokenIssuer, "mpit_auth_bot")
	svc.now = func() time.Time { return now }

	session, err := svc.LoginWithMAX(context.Background(), " user=%7B%7D ")
	if err != nil {
		t.Fatalf("LoginWithMAX() error = %v", err)
	}

	if authRepo.lastAuthParams.Provider != "max" {
		t.Fatalf("provider = %q, want max", authRepo.lastAuthParams.Provider)
	}

	if authRepo.lastAuthParams.ProviderUserID != "max-123" {
		t.Fatalf("provider user id = %q, want max-123", authRepo.lastAuthParams.ProviderUserID)
	}

	if authRepo.lastAuthParams.FullName != "Иван Иванов" {
		t.Fatalf("full name = %q, want Иван Иванов", authRepo.lastAuthParams.FullName)
	}

	if authRepo.lastAuthParams.Phone == nil || *authRepo.lastAuthParams.Phone != "79141010000" {
		t.Fatalf("phone = %v, want 79141010000", authRepo.lastAuthParams.Phone)
	}

	if tokenIssuer.lastUserID != "user-1" {
		t.Fatalf("issued for user id = %q, want user-1", tokenIssuer.lastUserID)
	}

	if session.AccessToken != "jwt-token" {
		t.Fatalf("access token = %q, want jwt-token", session.AccessToken)
	}
}

func TestAuthServiceStartMAXAuthCreatesSessionAndLink(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_900_000_100, 0).UTC()
	authRepo := &stubAuthRepository{
		createSessionResult: core.MAXAuthSession{
			SessionID: "session-123",
			Status:    core.MAXAuthSessionStatusPending,
			ExpiresAt: now.Add(maxAuthSessionTTL),
		},
	}

	svc := NewAuthService(authRepo, &stubMAXProvider{}, &stubTokenIssuer{}, "@mpit_auth_bot")
	svc.now = func() time.Time { return now }

	start, err := svc.StartMAXAuth(context.Background())
	if err != nil {
		t.Fatalf("StartMAXAuth() error = %v", err)
	}

	if authRepo.lastCreateSessionParams.ExpiresAt != now.Add(maxAuthSessionTTL) {
		t.Fatalf("expires at = %v, want %v", authRepo.lastCreateSessionParams.ExpiresAt, now.Add(maxAuthSessionTTL))
	}

	if start.MaxLink != "https://max.ru/mpit_auth_bot?startapp=session-123" {
		t.Fatalf("max link = %q", start.MaxLink)
	}
}

func TestAuthServiceCompleteMAXAuthMarksSessionCompleted(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_900_000_100, 0).UTC()
	authRepo := &stubAuthRepository{
		getSessionResult: core.MAXAuthSession{
			SessionID: "session-123",
			Status:    core.MAXAuthSessionStatusPending,
			ExpiresAt: now.Add(time.Minute),
		},
		authResult: core.User{
			ID:       "user-1",
			FullName: "Иван Иванов",
		},
		completeSessionResult: core.MAXAuthSession{
			SessionID:      "session-123",
			Status:         core.MAXAuthSessionStatusCompleted,
			ExpiresAt:      now.Add(time.Minute),
			CompletedAt:    timePtr(now),
			UserID:         "user-1",
			ProviderUserID: "max-123",
		},
	}
	maxProvider := &stubMAXProvider{
		identity: MAXIdentity{
			ProviderUserID: "max-123",
			FullName:       "Иван Иванов",
			StartParam:     "session-123",
			AuthDate:       now,
		},
	}

	svc := NewAuthService(authRepo, maxProvider, &stubTokenIssuer{}, "mpit_auth_bot")
	svc.now = func() time.Time { return now }

	session, err := svc.CompleteMAXAuth(context.Background(), "session-123", "init-data")
	if err != nil {
		t.Fatalf("CompleteMAXAuth() error = %v", err)
	}

	if authRepo.lastCompleteSessionParams.SessionID != "session-123" {
		t.Fatalf("session id = %q, want session-123", authRepo.lastCompleteSessionParams.SessionID)
	}
	if authRepo.lastCompleteSessionParams.UserID != "user-1" {
		t.Fatalf("user id = %q, want user-1", authRepo.lastCompleteSessionParams.UserID)
	}
	if session.Status != core.MAXAuthSessionStatusCompleted {
		t.Fatalf("status = %q, want completed", session.Status)
	}
}

func TestAuthServiceCompleteMAXAuthRejectsMismatchedStartParam(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_900_000_100, 0).UTC()
	authRepo := &stubAuthRepository{
		getSessionResult: core.MAXAuthSession{
			SessionID: "session-123",
			Status:    core.MAXAuthSessionStatusPending,
			ExpiresAt: now.Add(time.Minute),
		},
	}
	maxProvider := &stubMAXProvider{
		identity: MAXIdentity{
			ProviderUserID: "max-123",
			StartParam:     "another-session",
			AuthDate:       now,
		},
	}

	svc := NewAuthService(authRepo, maxProvider, &stubTokenIssuer{}, "mpit_auth_bot")
	svc.now = func() time.Time { return now }

	_, err := svc.CompleteMAXAuth(context.Background(), "session-123", "init-data")
	if !errors.Is(err, errorsstatus.ErrUnauthorized) || !errors.Is(err, ErrMAXStartParamMismatch) {
		t.Fatalf("error = %v, want unauthorized + start param mismatch", err)
	}
}

func TestAuthServiceExchangeMAXAuthReturnsJWT(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_900_000_100, 0).UTC()
	authRepo := &stubAuthRepository{
		getSessionResult: core.MAXAuthSession{
			SessionID:      "session-123",
			Status:         core.MAXAuthSessionStatusCompleted,
			ExpiresAt:      now.Add(time.Minute),
			CompletedAt:    timePtr(now.Add(-time.Second)),
			UserID:         "user-1",
			ProviderUserID: "max-123",
		},
		exchangeSessionResult: core.MAXAuthSession{
			SessionID:      "session-123",
			Status:         core.MAXAuthSessionStatusExchanged,
			ExpiresAt:      now.Add(time.Minute),
			CompletedAt:    timePtr(now.Add(-time.Second)),
			ExchangedAt:    timePtr(now),
			UserID:         "user-1",
			ProviderUserID: "max-123",
		},
		getUserByIDResult: core.User{
			ID:       "user-1",
			FullName: "Иван Иванов",
		},
	}
	tokenIssuer := &stubTokenIssuer{
		token:     "jwt-token",
		expiresAt: now.Add(time.Hour).Unix(),
	}

	svc := NewAuthService(authRepo, &stubMAXProvider{}, tokenIssuer, "mpit_auth_bot")
	svc.now = func() time.Time { return now }

	session, err := svc.ExchangeMAXAuth(context.Background(), "session-123")
	if err != nil {
		t.Fatalf("ExchangeMAXAuth() error = %v", err)
	}

	if authRepo.lastExchangeSessionParams.SessionID != "session-123" {
		t.Fatalf("session id = %q, want session-123", authRepo.lastExchangeSessionParams.SessionID)
	}
	if tokenIssuer.lastUserID != "user-1" {
		t.Fatalf("issued for user id = %q, want user-1", tokenIssuer.lastUserID)
	}
	if session.AccessToken != "jwt-token" {
		t.Fatalf("access token = %q, want jwt-token", session.AccessToken)
	}
}

func TestAuthServiceExchangeMAXAuthRejectsPendingSession(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_900_000_100, 0).UTC()
	authRepo := &stubAuthRepository{
		getSessionResult: core.MAXAuthSession{
			SessionID: "session-123",
			Status:    core.MAXAuthSessionStatusPending,
			ExpiresAt: now.Add(time.Minute),
		},
	}

	svc := NewAuthService(authRepo, &stubMAXProvider{}, &stubTokenIssuer{}, "mpit_auth_bot")
	svc.now = func() time.Time { return now }

	_, err := svc.ExchangeMAXAuth(context.Background(), "session-123")
	if !errors.Is(err, errorsstatus.ErrConflict) || !errors.Is(err, ErrMAXAuthSessionNotReady) {
		t.Fatalf("error = %v, want conflict + not ready", err)
	}
}

func TestAuthServiceUpdateProfileNormalizesPhone(t *testing.T) {
	t.Parallel()

	authRepo := &stubAuthRepository{
		updateResult: core.User{
			ID:       "user-3",
			FullName: "Иван Петров",
			Phone:    strPtr("79141010000"),
		},
	}
	svc := NewAuthService(authRepo, &stubMAXProvider{}, &stubTokenIssuer{}, "mpit_auth_bot")

	fullName := "  Иван Петров  "
	phone := "89141010000"

	user, err := svc.UpdateProfile(context.Background(), "user-3", UpdateProfileInput{
		FullName: &fullName,
		Phone:    &phone,
	})
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}

	if authRepo.lastUpdateParams.FullName == nil || *authRepo.lastUpdateParams.FullName != "Иван Петров" {
		t.Fatalf("full name = %v, want Иван Петров", authRepo.lastUpdateParams.FullName)
	}

	if authRepo.lastUpdateParams.Phone == nil || *authRepo.lastUpdateParams.Phone != "79141010000" {
		t.Fatalf("phone = %v, want 79141010000", authRepo.lastUpdateParams.Phone)
	}

	if user.ID != "user-3" {
		t.Fatalf("user id = %q, want user-3", user.ID)
	}
}

type stubAuthRepository struct {
	authResult                core.User
	authErr                   error
	updateResult              core.User
	updateErr                 error
	createSessionResult       core.MAXAuthSession
	createSessionErr          error
	getSessionResult          core.MAXAuthSession
	getSessionErr             error
	completeSessionResult     core.MAXAuthSession
	completeSessionErr        error
	exchangeSessionResult     core.MAXAuthSession
	exchangeSessionErr        error
	getUserByIDResult         core.User
	getUserByIDErr            error
	lastAuthParams            repo.AuthenticateWithOAuthParams
	lastUpdateParams          repo.UpdateUserProfileParams
	lastCreateSessionParams   repo.CreateMAXAuthSessionParams
	lastCompleteSessionParams repo.CompleteMAXAuthSessionParams
	lastExchangeSessionParams repo.ExchangeMAXAuthSessionParams
}

func (s *stubAuthRepository) AuthenticateWithOAuth(_ context.Context, params repo.AuthenticateWithOAuthParams) (core.User, error) {
	s.lastAuthParams = params
	if s.authErr != nil {
		return core.User{}, s.authErr
	}

	return s.authResult, nil
}

func (s *stubAuthRepository) CreateMAXAuthSession(_ context.Context, params repo.CreateMAXAuthSessionParams) (core.MAXAuthSession, error) {
	s.lastCreateSessionParams = params
	if s.createSessionErr != nil {
		return core.MAXAuthSession{}, s.createSessionErr
	}

	return s.createSessionResult, nil
}

func (s *stubAuthRepository) GetMAXAuthSession(_ context.Context, _ string) (core.MAXAuthSession, error) {
	if s.getSessionErr != nil {
		return core.MAXAuthSession{}, s.getSessionErr
	}

	return s.getSessionResult, nil
}

func (s *stubAuthRepository) CompleteMAXAuthSession(_ context.Context, params repo.CompleteMAXAuthSessionParams) (core.MAXAuthSession, error) {
	s.lastCompleteSessionParams = params
	if s.completeSessionErr != nil {
		return core.MAXAuthSession{}, s.completeSessionErr
	}

	return s.completeSessionResult, nil
}

func (s *stubAuthRepository) ExchangeMAXAuthSession(_ context.Context, params repo.ExchangeMAXAuthSessionParams) (core.MAXAuthSession, error) {
	s.lastExchangeSessionParams = params
	if s.exchangeSessionErr != nil {
		return core.MAXAuthSession{}, s.exchangeSessionErr
	}

	return s.exchangeSessionResult, nil
}

func (s *stubAuthRepository) GetUserByID(_ context.Context, _ string) (core.User, error) {
	if s.getUserByIDErr != nil {
		return core.User{}, s.getUserByIDErr
	}

	return s.getUserByIDResult, nil
}

func (s *stubAuthRepository) UpdateUserProfile(_ context.Context, params repo.UpdateUserProfileParams) (core.User, error) {
	s.lastUpdateParams = params
	if s.updateErr != nil {
		return core.User{}, s.updateErr
	}

	return s.updateResult, nil
}

type stubMAXProvider struct {
	identity MAXIdentity
	err      error
}

func (s *stubMAXProvider) ValidateInitData(_ context.Context, _ string) (MAXIdentity, error) {
	if s.err != nil {
		return MAXIdentity{}, s.err
	}

	return s.identity, nil
}

type stubTokenIssuer struct {
	token      string
	expiresAt  int64
	err        error
	lastUserID string
}

func (s *stubTokenIssuer) Issue(userID string) (string, int64, error) {
	s.lastUserID = userID
	if s.err != nil {
		return "", 0, s.err
	}

	return s.token, s.expiresAt, nil
}

func strPtr(value string) *string {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}
