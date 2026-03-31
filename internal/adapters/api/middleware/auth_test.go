package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddlewarePassesUserIDToContext(t *testing.T) {
	t.Parallel()

	handler := Auth(func(token string) (string, error) {
		if token != "good-token" {
			t.Fatalf("token = %q, want good-token", token)
		}
		return "user-1", nil
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("user id missing from context")
		}
		if userID != "user-1" {
			t.Fatalf("user id = %q, want user-1", userID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/my", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestAuthMiddlewareRejectsMissingToken(t *testing.T) {
	t.Parallel()

	handler := Auth(func(token string) (string, error) {
		return "user-1", nil
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/my", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareRejectsVerifierError(t *testing.T) {
	t.Parallel()

	handler := Auth(func(token string) (string, error) {
		return "", errors.New("bad token")
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/my", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
