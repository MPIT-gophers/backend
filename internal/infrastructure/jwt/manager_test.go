package jwt

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	errorsstatus "eventAI/internal/errorsStatus"
)

func TestManagerVerifyAcceptsValidToken(t *testing.T) {
	t.Parallel()

	manager, err := NewManager("secret", time.Hour)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	manager.now = func() time.Time {
		return time.Unix(1_700_000_000, 0).UTC()
	}

	token := signedToken(t, manager, Claims{
		Subject:   "user-1",
		IssuedAt:  1_700_000_000,
		ExpiresAt: 1_700_000_300,
	})

	claims, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if claims.Subject != "user-1" {
		t.Fatalf("subject = %q, want user-1", claims.Subject)
	}
}

func TestManagerVerifyRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	manager, err := NewManager("secret", time.Hour)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	manager.now = func() time.Time {
		return time.Unix(1_700_000_500, 0).UTC()
	}

	token := signedToken(t, manager, Claims{
		Subject:   "user-1",
		IssuedAt:  1_700_000_000,
		ExpiresAt: 1_700_000_300,
	})

	_, err = manager.Verify(token)
	if !errors.Is(err, errorsstatus.ErrUnauthorized) {
		t.Fatalf("error = %v, want unauthorized", err)
	}
}

func TestManagerVerifyRejectsTamperedSignature(t *testing.T) {
	t.Parallel()

	manager, err := NewManager("secret", time.Hour)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	manager.now = func() time.Time {
		return time.Unix(1_700_000_000, 0).UTC()
	}

	token := signedToken(t, manager, Claims{
		Subject:   "user-1",
		IssuedAt:  1_700_000_000,
		ExpiresAt: 1_700_000_300,
	})

	_, err = manager.Verify(token + "broken")
	if !errors.Is(err, errorsstatus.ErrUnauthorized) {
		t.Fatalf("error = %v, want unauthorized", err)
	}
}

func TestManagerIssueAndVerify(t *testing.T) {
	t.Parallel()

	manager, err := NewManager("secret", time.Hour)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	manager.now = func() time.Time {
		return time.Unix(1_700_000_000, 0).UTC()
	}

	token, expUnix, err := manager.Issue("user-42")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if expUnix != 1_700_003_600 {
		t.Fatalf("exp unix = %d, want 1700003600", expUnix)
	}

	claims, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if claims.Subject != "user-42" {
		t.Fatalf("subject = %q, want user-42", claims.Subject)
	}
}

func signedToken(t *testing.T, manager *Manager, claims Claims) string {
	t.Helper()

	header, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	headerEncoded := base64.RawURLEncoding.EncodeToString(header)
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := headerEncoded + "." + payloadEncoded
	signature := base64.RawURLEncoding.EncodeToString(manager.sign(signingInput))

	return signingInput + "." + signature
}
