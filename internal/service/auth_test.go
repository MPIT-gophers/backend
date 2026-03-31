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
		},
	}
	tokenIssuer := &stubTokenIssuer{
		token:     "jwt-token",
		expiresAt: time.Unix(1_900_000_000, 0).UTC().Unix(),
	}

	svc := NewAuthService(authRepo, maxProvider, tokenIssuer)

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

	if session.User.ID != "user-1" {
		t.Fatalf("session user id = %q, want user-1", session.User.ID)
	}
}

func TestAuthServiceLoginWithMAXUsesPlaceholderAndNilPhone(t *testing.T) {
	t.Parallel()

	authRepo := &stubAuthRepository{
		authResult: core.User{
			ID:       "user-2",
			FullName: UnsetFullName,
		},
	}
	maxProvider := &stubMAXProvider{
		identity: MAXIdentity{
			ProviderUserID: "max-456",
		},
	}
	tokenIssuer := &stubTokenIssuer{
		token:     "jwt-token",
		expiresAt: time.Unix(1_900_000_001, 0).UTC().Unix(),
	}

	svc := NewAuthService(authRepo, maxProvider, tokenIssuer)

	_, err := svc.LoginWithMAX(context.Background(), "user=%7B%7D")
	if err != nil {
		t.Fatalf("LoginWithMAX() error = %v", err)
	}

	if authRepo.lastAuthParams.FullName != UnsetFullName {
		t.Fatalf("full name = %q, want %q", authRepo.lastAuthParams.FullName, UnsetFullName)
	}

	if authRepo.lastAuthParams.Phone != nil {
		t.Fatalf("phone = %v, want nil", authRepo.lastAuthParams.Phone)
	}
}

func TestAuthServiceLoginWithMAXPropagatesUnauthorized(t *testing.T) {
	t.Parallel()

	svc := NewAuthService(
		&stubAuthRepository{},
		&stubMAXProvider{err: errorsstatus.ErrUnauthorized},
		&stubTokenIssuer{},
	)

	_, err := svc.LoginWithMAX(context.Background(), "user=%7B%7D")
	if !errors.Is(err, errorsstatus.ErrUnauthorized) {
		t.Fatalf("error = %v, want unauthorized", err)
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
	svc := NewAuthService(authRepo, &stubMAXProvider{}, &stubTokenIssuer{})

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

func TestAuthServiceUpdateProfileRejectsInvalidPhone(t *testing.T) {
	t.Parallel()

	svc := NewAuthService(&stubAuthRepository{}, &stubMAXProvider{}, &stubTokenIssuer{})
	phone := "+123"

	_, err := svc.UpdateProfile(context.Background(), "user-4", UpdateProfileInput{
		Phone: &phone,
	})
	if !errors.Is(err, errorsstatus.ErrInvalidInput) {
		t.Fatalf("error = %v, want invalid input", err)
	}
}

type stubAuthRepository struct {
	authResult       core.User
	authErr          error
	updateResult     core.User
	updateErr        error
	lastAuthParams   repo.AuthenticateWithOAuthParams
	lastUpdateParams repo.UpdateUserProfileParams
}

func (s *stubAuthRepository) AuthenticateWithOAuth(_ context.Context, params repo.AuthenticateWithOAuthParams) (core.User, error) {
	s.lastAuthParams = params
	if s.authErr != nil {
		return core.User{}, s.authErr
	}

	return s.authResult, nil
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
