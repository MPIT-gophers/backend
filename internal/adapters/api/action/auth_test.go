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

	"github.com/go-chi/chi/v5"
)

type authSessionResponse struct {
	Data core.AuthSession `json:"data"`
}

type maxAuthStartResponse struct {
	Data core.MAXAuthStart `json:"data"`
}

type maxAuthSessionResponse struct {
	Data core.MAXAuthSession `json:"data"`
}

type userResponse struct {
	Data core.User `json:"data"`
}

func TestAuthHandlerStartMAXAuthSuccess(t *testing.T) {
	t.Parallel()

	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	handler := NewAuthHandler(service.NewAuthService(
		&stubAuthRepository{
			createSessionResult: core.MAXAuthSession{
				SessionID: "session-123",
				Status:    core.MAXAuthSessionStatusPending,
				ExpiresAt: expiresAt,
			},
		},
		&stubMAXProvider{},
		&stubTokenIssuer{},
		"mpit_auth_bot",
	))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/max/start", nil)
	rec := httptest.NewRecorder()

	handler.StartMAXAuth(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var response maxAuthStartResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.MaxLink != "https://max.ru/mpit_auth_bot?startapp=session-123" {
		t.Fatalf("max link = %q", response.Data.MaxLink)
	}
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
				AuthDate:       time.Now().UTC(),
			},
		},
		&stubTokenIssuer{
			token:     "jwt-token",
			expiresAt: time.Unix(1_900_000_000, 0).UTC().Unix(),
		},
		"mpit_auth_bot",
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
}

func TestAuthHandlerCompleteMAXAuthSuccess(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	handler := NewAuthHandler(service.NewAuthService(
		&stubAuthRepository{
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
				SessionID:   "session-123",
				Status:      core.MAXAuthSessionStatusCompleted,
				ExpiresAt:   now.Add(time.Minute),
				CompletedAt: timePtr(now),
				UserID:      "user-1",
			},
		},
		&stubMAXProvider{
			identity: service.MAXIdentity{
				ProviderUserID: "max-123",
				FullName:       "Иван Иванов",
				StartParam:     "session-123",
				AuthDate:       now,
			},
		},
		&stubTokenIssuer{},
		"mpit_auth_bot",
	))

	body := bytes.NewBufferString(`{"session_id":"session-123","init_data":"user=%7B%7D&hash=test"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/max/complete", body)
	rec := httptest.NewRecorder()

	handler.CompleteMAXAuth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var response maxAuthSessionResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.Status != core.MAXAuthSessionStatusCompleted {
		t.Fatalf("status = %q, want completed", response.Data.Status)
	}
}

func TestAuthHandlerGetMAXAuthSessionSuccess(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(service.NewAuthService(
		&stubAuthRepository{
			getSessionResult: core.MAXAuthSession{
				SessionID: "session-123",
				Status:    core.MAXAuthSessionStatusPending,
				ExpiresAt: time.Now().UTC().Add(time.Minute),
			},
		},
		&stubMAXProvider{},
		&stubTokenIssuer{},
		"mpit_auth_bot",
	))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/max/session/session-123", nil)
	rec := httptest.NewRecorder()
	req = withURLParam(req, "sessionID", "session-123")

	handler.GetMAXAuthSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthHandlerExchangeMAXAuthSuccess(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	handler := NewAuthHandler(service.NewAuthService(
		&stubAuthRepository{
			getSessionResult: core.MAXAuthSession{
				SessionID:   "session-123",
				Status:      core.MAXAuthSessionStatusCompleted,
				ExpiresAt:   now.Add(time.Minute),
				CompletedAt: timePtr(now.Add(-time.Second)),
				UserID:      "user-1",
			},
			exchangeSessionResult: core.MAXAuthSession{
				SessionID:   "session-123",
				Status:      core.MAXAuthSessionStatusExchanged,
				ExpiresAt:   now.Add(time.Minute),
				CompletedAt: timePtr(now.Add(-time.Second)),
				ExchangedAt: timePtr(now),
				UserID:      "user-1",
			},
			getUserByIDResult: core.User{
				ID:       "user-1",
				FullName: "Иван Иванов",
			},
		},
		&stubMAXProvider{},
		&stubTokenIssuer{
			token:     "jwt-token",
			expiresAt: now.Add(time.Hour).Unix(),
		},
		"mpit_auth_bot",
	))

	body := bytes.NewBufferString(`{"session_id":"session-123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/max/exchange", body)
	rec := httptest.NewRecorder()

	handler.ExchangeMAXAuth(rec, req)

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
}

func TestAuthHandlerLoginWithMAXInvalidJSON(t *testing.T) {
	t.Parallel()

	handler := NewAuthHandler(service.NewAuthService(
		&stubAuthRepository{},
		&stubMAXProvider{},
		&stubTokenIssuer{},
		"mpit_auth_bot",
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
		"mpit_auth_bot",
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
	authResult            core.User
	updateResult          core.User
	createSessionResult   core.MAXAuthSession
	getSessionResult      core.MAXAuthSession
	completeSessionResult core.MAXAuthSession
	exchangeSessionResult core.MAXAuthSession
	getUserByIDResult     core.User
}

func (s *stubAuthRepository) AuthenticateWithOAuth(_ context.Context, _ repo.AuthenticateWithOAuthParams) (core.User, error) {
	return s.authResult, nil
}

func (s *stubAuthRepository) CreateMAXAuthSession(_ context.Context, _ repo.CreateMAXAuthSessionParams) (core.MAXAuthSession, error) {
	return s.createSessionResult, nil
}

func (s *stubAuthRepository) GetMAXAuthSession(_ context.Context, _ string) (core.MAXAuthSession, error) {
	return s.getSessionResult, nil
}

func (s *stubAuthRepository) CompleteMAXAuthSession(_ context.Context, _ repo.CompleteMAXAuthSessionParams) (core.MAXAuthSession, error) {
	return s.completeSessionResult, nil
}

func (s *stubAuthRepository) ExchangeMAXAuthSession(_ context.Context, _ repo.ExchangeMAXAuthSessionParams) (core.MAXAuthSession, error) {
	return s.exchangeSessionResult, nil
}

func (s *stubAuthRepository) GetUserByID(_ context.Context, _ string) (core.User, error) {
	return s.getUserByIDResult, nil
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

func withURLParam(req *http.Request, key string, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}

func strPtr(value string) *string {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}
