package repo

import (
	"context"
	"time"

	"eventAI/internal/entities/core"
)

type AuthenticateWithOAuthParams struct {
	Provider       string
	ProviderUserID string
	FullName       string
	Phone          *string
}

type UpdateUserProfileParams struct {
	UserID      string
	FullName    *string
	FullNameSet bool
	Phone       *string
	PhoneSet    bool
}

type CreateMAXAuthSessionParams struct {
	ExpiresAt time.Time
}

type CompleteMAXAuthSessionParams struct {
	SessionID      string
	ProviderUserID string
	UserID         string
	CompletedAt    time.Time
}

type ExchangeMAXAuthSessionParams struct {
	SessionID   string
	ExchangedAt time.Time
}

type AuthRepository interface {
	AuthenticateWithOAuth(ctx context.Context, params AuthenticateWithOAuthParams) (core.User, error)
	CreateMAXAuthSession(ctx context.Context, params CreateMAXAuthSessionParams) (core.MAXAuthSession, error)
	GetMAXAuthSession(ctx context.Context, sessionID string) (core.MAXAuthSession, error)
	CompleteMAXAuthSession(ctx context.Context, params CompleteMAXAuthSessionParams) (core.MAXAuthSession, error)
	ExchangeMAXAuthSession(ctx context.Context, params ExchangeMAXAuthSessionParams) (core.MAXAuthSession, error)
	GetUserByID(ctx context.Context, userID string) (core.User, error)
	UpdateUserProfile(ctx context.Context, params UpdateUserProfileParams) (core.User, error)
}
