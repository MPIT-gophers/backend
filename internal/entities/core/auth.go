package core

import "time"

const (
	MAXAuthSessionStatusPending   = "pending"
	MAXAuthSessionStatusCompleted = "completed"
	MAXAuthSessionStatusExchanged = "exchanged"
	MAXAuthSessionStatusExpired   = "expired"
	MAXAuthSessionStatusFailed    = "failed"
)

type MAXAuthSession struct {
	SessionID      string     `json:"session_id"`
	Status         string     `json:"status"`
	ExpiresAt      time.Time  `json:"expires_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	ExchangedAt    *time.Time `json:"exchanged_at,omitempty"`
	UserID         string     `json:"-"`
	ProviderUserID string     `json:"-"`
}

type MAXAuthStart struct {
	SessionID string    `json:"session_id"`
	MaxLink   string    `json:"max_link"`
	ExpiresAt time.Time `json:"expires_at"`
}
