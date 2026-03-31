package service

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"eventAI/internal/entities/core"
	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/repo"
)

const (
	UnsetFullName      = "Не заполнено"
	maxAuthSessionTTL  = 10 * time.Minute
	maxInitDataMaxAge  = 10 * time.Minute
	maxDeepLinkBaseURL = "https://max.ru/"
	maxAuthProvider    = "max"
)

var (
	ErrMAXAuthSessionNotReady         = errors.New("max auth session not ready")
	ErrMAXAuthSessionExpired          = errors.New("max auth session expired")
	ErrMAXAuthSessionAlreadyExchanged = errors.New("max auth session already exchanged")
	ErrMAXAuthSessionAlreadyUsed      = errors.New("max auth session already completed")
	ErrMAXInitDataExpired             = errors.New("max init data expired")
	ErrMAXStartParamMismatch          = errors.New("max start param mismatch")
)

type MAXIdentity struct {
	ProviderUserID string
	FullName       string
	Phone          *string
	StartParam     string
	AuthDate       time.Time
}

type MAXIdentityProvider interface {
	ValidateInitData(ctx context.Context, initData string) (MAXIdentity, error)
}

type TokenIssuer interface {
	Issue(userID string) (token string, expiresAtUnix int64, err error)
}

type AuthService struct {
	repo           repo.AuthRepository
	max            MAXIdentityProvider
	issuer         TokenIssuer
	maxBotUsername string
	now            func() time.Time
}

func NewAuthService(repo repo.AuthRepository, max MAXIdentityProvider, issuer TokenIssuer, maxBotUsername string) *AuthService {
	return &AuthService{
		repo:           repo,
		max:            max,
		issuer:         issuer,
		maxBotUsername: normalizeBotUsername(maxBotUsername),
		now:            time.Now,
	}
}

func (s *AuthService) StartMAXAuth(ctx context.Context) (core.MAXAuthStart, error) {
	if s.maxBotUsername == "" {
		return core.MAXAuthStart{}, errorsstatus.ErrServiceUnavailable
	}

	session, err := s.repo.CreateMAXAuthSession(ctx, repo.CreateMAXAuthSessionParams{
		ExpiresAt: s.now().UTC().Add(maxAuthSessionTTL),
	})
	if err != nil {
		return core.MAXAuthStart{}, err
	}

	return core.MAXAuthStart{
		SessionID: session.SessionID,
		MaxLink:   maxDeepLinkBaseURL + s.maxBotUsername + "?startapp=" + url.QueryEscape(session.SessionID),
		ExpiresAt: session.ExpiresAt,
	}, nil
}

func (s *AuthService) GetMAXAuthSession(ctx context.Context, sessionID string) (core.MAXAuthSession, error) {
	session, err := s.repo.GetMAXAuthSession(ctx, sessionID)
	if err != nil {
		return core.MAXAuthSession{}, err
	}

	return s.withDerivedStatus(session), nil
}

func (s *AuthService) CompleteMAXAuth(ctx context.Context, sessionID string, initData string) (core.MAXAuthSession, error) {
	session, err := s.repo.GetMAXAuthSession(ctx, sessionID)
	if err != nil {
		return core.MAXAuthSession{}, err
	}

	session = s.withDerivedStatus(session)
	if err := validateCompletableSession(session); err != nil {
		return core.MAXAuthSession{}, err
	}

	identity, err := s.validateMAXIdentity(ctx, initData)
	if err != nil {
		return core.MAXAuthSession{}, err
	}

	if strings.TrimSpace(identity.StartParam) != session.SessionID {
		return core.MAXAuthSession{}, errors.Join(errorsstatus.ErrUnauthorized, ErrMAXStartParamMismatch)
	}

	user, err := s.authenticateMAXIdentity(ctx, identity)
	if err != nil {
		return core.MAXAuthSession{}, err
	}

	completedAt := s.now().UTC()
	session, err = s.repo.CompleteMAXAuthSession(ctx, repo.CompleteMAXAuthSessionParams{
		SessionID:      session.SessionID,
		ProviderUserID: identity.ProviderUserID,
		UserID:         user.ID,
		CompletedAt:    completedAt,
	})
	if err != nil {
		if errors.Is(err, errorsstatus.ErrConflict) {
			return core.MAXAuthSession{}, errors.Join(errorsstatus.ErrConflict, ErrMAXAuthSessionAlreadyUsed)
		}
		return core.MAXAuthSession{}, err
	}

	return s.withDerivedStatus(session), nil
}

func (s *AuthService) ExchangeMAXAuth(ctx context.Context, sessionID string) (core.AuthSession, error) {
	session, err := s.repo.GetMAXAuthSession(ctx, sessionID)
	if err != nil {
		return core.AuthSession{}, err
	}

	session = s.withDerivedStatus(session)
	switch session.Status {
	case core.MAXAuthSessionStatusPending:
		return core.AuthSession{}, errors.Join(errorsstatus.ErrConflict, ErrMAXAuthSessionNotReady)
	case core.MAXAuthSessionStatusExpired:
		return core.AuthSession{}, errors.Join(errorsstatus.ErrConflict, ErrMAXAuthSessionExpired)
	case core.MAXAuthSessionStatusExchanged:
		return core.AuthSession{}, errors.Join(errorsstatus.ErrConflict, ErrMAXAuthSessionAlreadyExchanged)
	case core.MAXAuthSessionStatusCompleted:
	default:
		return core.AuthSession{}, errors.Join(errorsstatus.ErrConflict, ErrMAXAuthSessionAlreadyUsed)
	}

	exchangedAt := s.now().UTC()
	exchangedSession, err := s.repo.ExchangeMAXAuthSession(ctx, repo.ExchangeMAXAuthSessionParams{
		SessionID:   session.SessionID,
		ExchangedAt: exchangedAt,
	})
	if err != nil {
		if errors.Is(err, errorsstatus.ErrConflict) {
			return core.AuthSession{}, errors.Join(errorsstatus.ErrConflict, ErrMAXAuthSessionAlreadyExchanged)
		}
		return core.AuthSession{}, err
	}

	if strings.TrimSpace(exchangedSession.UserID) == "" {
		return core.AuthSession{}, errors.Join(errorsstatus.ErrConflict, ErrMAXAuthSessionNotReady)
	}

	user, err := s.repo.GetUserByID(ctx, exchangedSession.UserID)
	if err != nil {
		return core.AuthSession{}, err
	}

	return s.issueAuthSession(user)
}

func (s *AuthService) LoginWithMAX(ctx context.Context, initData string) (core.AuthSession, error) {
	identity, err := s.validateMAXIdentity(ctx, initData)
	if err != nil {
		return core.AuthSession{}, err
	}

	user, err := s.authenticateMAXIdentity(ctx, identity)
	if err != nil {
		return core.AuthSession{}, err
	}

	return s.issueAuthSession(user)
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

func (s *AuthService) validateMAXIdentity(ctx context.Context, initData string) (MAXIdentity, error) {
	initData = strings.TrimSpace(initData)
	if initData == "" {
		return MAXIdentity{}, errorsstatus.ErrInvalidInput
	}

	identity, err := s.max.ValidateInitData(ctx, initData)
	if err != nil {
		return MAXIdentity{}, err
	}

	if err := validateMAXIdentityFreshness(s.now().UTC(), identity); err != nil {
		return MAXIdentity{}, err
	}

	return identity, nil
}

func (s *AuthService) authenticateMAXIdentity(ctx context.Context, identity MAXIdentity) (core.User, error) {
	providerUserID := strings.TrimSpace(identity.ProviderUserID)
	if providerUserID == "" {
		return core.User{}, errorsstatus.ErrUnauthorized
	}

	fullName := normalizeProviderFullName(identity.FullName)
	phone := normalizeProviderPhone(identity.Phone)

	return s.repo.AuthenticateWithOAuth(ctx, repo.AuthenticateWithOAuthParams{
		Provider:       maxAuthProvider,
		ProviderUserID: providerUserID,
		FullName:       fullName,
		Phone:          phone,
	})
}

func (s *AuthService) issueAuthSession(user core.User) (core.AuthSession, error) {
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

func (s *AuthService) withDerivedStatus(session core.MAXAuthSession) core.MAXAuthSession {
	if session.Status == core.MAXAuthSessionStatusPending && !session.ExpiresAt.After(s.now().UTC()) {
		session.Status = core.MAXAuthSessionStatusExpired
	}

	return session
}

func validateCompletableSession(session core.MAXAuthSession) error {
	switch session.Status {
	case core.MAXAuthSessionStatusPending:
		return nil
	case core.MAXAuthSessionStatusExpired:
		return errors.Join(errorsstatus.ErrConflict, ErrMAXAuthSessionExpired)
	case core.MAXAuthSessionStatusExchanged:
		return errors.Join(errorsstatus.ErrConflict, ErrMAXAuthSessionAlreadyExchanged)
	default:
		return errors.Join(errorsstatus.ErrConflict, ErrMAXAuthSessionAlreadyUsed)
	}
}

func validateMAXIdentityFreshness(now time.Time, identity MAXIdentity) error {
	if identity.AuthDate.IsZero() {
		return errorsstatus.ErrUnauthorized
	}

	authDate := identity.AuthDate.UTC()
	if authDate.After(now.Add(30 * time.Second)) {
		return errors.Join(errorsstatus.ErrUnauthorized, ErrMAXInitDataExpired)
	}
	if now.Sub(authDate) > maxInitDataMaxAge {
		return errors.Join(errorsstatus.ErrUnauthorized, ErrMAXInitDataExpired)
	}

	return nil
}

func normalizeBotUsername(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "@")
	return value
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
