package service

import (
	"context"
	"strings"
	"time"

	"eventAI/internal/entities/core"
	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/repo"
)

const UnsetFullName = "Не заполнено"

type MAXIdentity struct {
	ProviderUserID string
	FullName       string
	Phone          *string
}

type MAXIdentityProvider interface {
	ValidateToken(ctx context.Context, token string) (MAXIdentity, error)
}

type TokenIssuer interface {
	Issue(userID string) (token string, expiresAtUnix int64, err error)
}

type AuthService struct {
	repo   repo.AuthRepository
	max    MAXIdentityProvider
	issuer TokenIssuer
}

func NewAuthService(repo repo.AuthRepository, max MAXIdentityProvider, issuer TokenIssuer) *AuthService {
	return &AuthService{
		repo:   repo,
		max:    max,
		issuer: issuer,
	}
}

func (s *AuthService) LoginWithMAX(ctx context.Context, token string) (core.AuthSession, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return core.AuthSession{}, errorsstatus.ErrInvalidInput
	}

	identity, err := s.max.ValidateToken(ctx, token)
	if err != nil {
		return core.AuthSession{}, err
	}

	providerUserID := strings.TrimSpace(identity.ProviderUserID)
	if providerUserID == "" {
		return core.AuthSession{}, errorsstatus.ErrUnauthorized
	}

	fullName := normalizeProviderFullName(identity.FullName)
	phone := normalizeProviderPhone(identity.Phone)

	user, err := s.repo.AuthenticateWithOAuth(ctx, repo.AuthenticateWithOAuthParams{
		Provider:       "max",
		ProviderUserID: providerUserID,
		FullName:       fullName,
		Phone:          phone,
	})
	if err != nil {
		return core.AuthSession{}, err
	}

	accessToken, expiresAtUnix, err := s.issuer.Issue(user.ID)
	if err != nil {
		return core.AuthSession{}, err
	}

	return core.AuthSession{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresAt:   time.Unix(expiresAtUnix, 0).UTC(),
		User:        user,
	}, nil
}

type UpdateProfileInput struct {
	FullName *string
	Phone    *string
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID string, input UpdateProfileInput) (core.User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return core.User{}, errorsstatus.ErrUnauthorized
	}

	fullName, fullNameSet, err := normalizeUpdateFullName(input.FullName)
	if err != nil {
		return core.User{}, err
	}

	phone, phoneSet, err := normalizeUpdatePhone(input.Phone)
	if err != nil {
		return core.User{}, err
	}

	if !fullNameSet && !phoneSet {
		return core.User{}, errorsstatus.ErrInvalidInput
	}

	return s.repo.UpdateUserProfile(ctx, repo.UpdateUserProfileParams{
		UserID:      userID,
		FullName:    fullName,
		FullNameSet: fullNameSet,
		Phone:       phone,
		PhoneSet:    phoneSet,
	})
}

func normalizeProviderFullName(value string) string {
	value = strings.TrimSpace(value)
	length := len([]rune(value))
	if length < 2 || length > 255 {
		return UnsetFullName
	}

	return value
}

func normalizeUpdateFullName(value *string) (*string, bool, error) {
	if value == nil {
		return nil, false, nil
	}

	trimmed := strings.TrimSpace(*value)
	length := len([]rune(trimmed))
	if length < 2 || length > 255 {
		return nil, false, errorsstatus.ErrInvalidInput
	}

	return &trimmed, true, nil
}

func normalizeProviderPhone(value *string) *string {
	phone, err := normalizePhone(value)
	if err != nil {
		return nil
	}

	return phone
}

func normalizeUpdatePhone(value *string) (*string, bool, error) {
	if value == nil {
		return nil, false, nil
	}

	phone, err := normalizePhone(value)
	if err != nil {
		return nil, false, err
	}

	return phone, true, nil
}

func normalizePhone(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}

	raw := strings.TrimSpace(*value)
	if raw == "" {
		return nil, nil
	}

	digits := make([]rune, 0, len(raw))
	for _, ch := range raw {
		if ch >= '0' && ch <= '9' {
			digits = append(digits, ch)
		}
	}

	if len(digits) != 11 {
		return nil, errorsstatus.ErrInvalidInput
	}

	if digits[0] == '8' {
		digits[0] = '7'
	}

	if digits[0] != '7' {
		return nil, errorsstatus.ErrInvalidInput
	}

	normalized := string(digits)
	return &normalized, nil
}
