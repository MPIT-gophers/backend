package action

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"eventAI/internal/adapters/api/middleware"
	"eventAI/internal/entities/core"
	"eventAI/internal/repo"
	"eventAI/internal/service"
)

type authSessionResponse struct {
	Data core.AuthSession `json:"data"`
}

type userResponse struct {
	Data core.User `json:"data"`
}

func TestAuthHandlerLoginWithMAXSuccess(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(service.NewAuthService(
		&stubAuthRepository{
			authResult: core.User{
				ID:       "user-1",
				FullName: "Иван Иванов",
			},
		},
		&stubMAXProvider{
			identity: service.MAXIdentity{
				ProviderUserID: "max-123",
				FullName:       "Иван Иванов",
			},
		},
		&stubTokenIssuer{
			token:     "jwt-token",
			expiresAt: time.Unix(1_900_000_000, 0).UTC().Unix(),
		},
	))

	body := bytes.NewBufferString(`{"init_data":"user=%7B%7D&hash=test"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/max/login", body)
	rec := httptest.NewRecorder()

	handler.LoginWithMAX(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var response authSessionResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.AccessToken != "jwt-token" {
		t.Fatalf("access token = %q, want jwt-token", response.Data.AccessToken)
	}

	if response.Data.User.ID != "user-1" {
		t.Fatalf("user id = %q, want user-1", response.Data.User.ID)
	}
}

func TestAuthHandlerLoginWithMAXInvalidJSON(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(service.NewAuthService(
		&stubAuthRepository{},
		&stubMAXProvider{},
		&stubTokenIssuer{},
	))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/max/login", bytes.NewBufferString(`{"init_data":`))
	rec := httptest.NewRecorder()

	handler.LoginWithMAX(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAuthHandlerUpdateMeSuccess(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(service.NewAuthService(
		&stubAuthRepository{
			updateResult: core.User{
				ID:       "user-1",
				FullName: "Иван Петров",
				Phone:    strPtr("79141010000"),
			},
		},
		&stubMAXProvider{},
		&stubTokenIssuer{},
	))

	wrapped := middleware.Auth(func(token string) (string, error) {
		return "user-1", nil
	})(http.HandlerFunc(handler.UpdateMe))

	body := bytes.NewBufferString(`{"full_name":"Иван Петров","phone":"89141010000"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/me", body)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var response userResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.FullName != "Иван Петров" {
		t.Fatalf("full name = %q, want Иван Петров", response.Data.FullName)
	}
}

type stubAuthRepository struct {
	authResult   core.User
	updateResult core.User
}

func (s *stubAuthRepository) AuthenticateWithOAuth(_ context.Context, _ repo.AuthenticateWithOAuthParams) (core.User, error) {
	return s.authResult, nil
}

func (s *stubAuthRepository) UpdateUserProfile(_ context.Context, _ repo.UpdateUserProfileParams) (core.User, error) {
	return s.updateResult, nil
}

type stubMAXProvider struct {
	identity service.MAXIdentity
}

func (s *stubMAXProvider) ValidateInitData(_ context.Context, _ string) (service.MAXIdentity, error) {
	return s.identity, nil
}

type stubTokenIssuer struct {
	token     string
	expiresAt int64
}

func (s *stubTokenIssuer) Issue(_ string) (string, int64, error) {
	return s.token, s.expiresAt, nil
}

func strPtr(value string) *string {
	return &value
}
