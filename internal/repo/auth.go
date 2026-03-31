package repo

import (
	"context"

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

type AuthRepository interface {
	AuthenticateWithOAuth(ctx context.Context, params AuthenticateWithOAuthParams) (core.User, error)
	UpdateUserProfile(ctx context.Context, params UpdateUserProfileParams) (core.User, error)
}
