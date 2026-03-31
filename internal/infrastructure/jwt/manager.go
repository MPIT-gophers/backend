package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	errorsstatus "eventAI/internal/errorsStatus"
)

type Manager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

type Claims struct {
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func NewManager(secret string, ttl time.Duration) (*Manager, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, fmt.Errorf("jwt secret is required")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("jwt ttl must be positive")
	}

	return &Manager{
		secret: []byte(secret),
		ttl:    ttl,
		now:    time.Now,
	}, nil
}

func (m *Manager) Issue(userID string) (string, int64, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", 0, errorsstatus.ErrUnauthorized
	}

	now := m.now().UTC()
	claims := Claims{
		Subject:   userID,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(m.ttl).Unix(),
	}

	token, err := m.signToken(claims)
	if err != nil {
		return "", 0, err
	}

	return token, claims.ExpiresAt, nil
}

func (m *Manager) Verify(token string) (Claims, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return Claims{}, errorsstatus.ErrUnauthorized
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSignature := m.sign(signingInput)
	actualSignature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, errorsstatus.ErrUnauthorized
	}

	if !hmac.Equal(actualSignature, expectedSignature) {
		return Claims{}, errorsstatus.ErrUnauthorized
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, errorsstatus.ErrUnauthorized
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, errorsstatus.ErrUnauthorized
	}

	if claims.Subject == "" || claims.ExpiresAt <= m.now().Unix() {
		return Claims{}, errorsstatus.ErrUnauthorized
	}

	return claims, nil
}

func (m *Manager) sign(signingInput string) []byte {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(signingInput))
	return mac.Sum(nil)
}

func (m *Manager) signToken(claims Claims) (string, error) {
	header, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	headerEncoded := base64.RawURLEncoding.EncodeToString(header)
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := headerEncoded + "." + payloadEncoded
	signature := base64.RawURLEncoding.EncodeToString(m.sign(signingInput))

	return signingInput + "." + signature, nil
}
