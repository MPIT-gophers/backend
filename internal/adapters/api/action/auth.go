package action

import (
	"encoding/json"
	"errors"
	"net/http"

	"eventAI/internal/adapters/api/middleware"
	"eventAI/internal/adapters/api/response"
	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/service"

	"github.com/go-chi/chi/v5"
)

type AuthHandler struct {
	service *service.AuthService
}

func NewAuthHandler(service *service.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

type LoginWithMAXRequest struct {
	InitData string `json:"init_data"`
}

type CompleteMAXAuthRequest struct {
	SessionID string `json:"session_id"`
	InitData  string `json:"init_data"`
}

type ExchangeMAXAuthRequest struct {
	SessionID string `json:"session_id"`
}

// StartMAXAuth godoc
// @Summary Start mobile auth flow through MAX mini app
// @Tags auth
// @Produce json
// @Success 201 {object} response.SuccessEnvelope{data=core.MAXAuthStart}
// @Failure 503 {object} response.ErrorEnvelope
// @Router /auth/max/start [post]
func (h *AuthHandler) StartMAXAuth(w http.ResponseWriter, r *http.Request) {
	start, err := h.service.StartMAXAuth(r.Context())
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), authErrorMessage(err))
		return
	}

	response.Success(w, http.StatusCreated, start)
}

// LoginWithMAX godoc
// @Summary Login with MAX WebApp initData
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginWithMAXRequest true "MAX login payload"
// @Success 200 {object} response.SuccessEnvelope{data=core.AuthSession}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 409 {object} response.ErrorEnvelope
// @Router /auth/max/login [post]
func (h *AuthHandler) LoginWithMAX(w http.ResponseWriter, r *http.Request) {
	var req LoginWithMAXRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid json body")
		return
	}

	session, err := h.service.LoginWithMAX(r.Context(), req.InitData)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), authErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, session)
}

// CompleteMAXAuth godoc
// @Summary Complete mobile auth flow through MAX initData
// @Tags auth
// @Accept json
// @Produce json
// @Param request body CompleteMAXAuthRequest true "MAX auth completion payload"
// @Success 200 {object} response.SuccessEnvelope{data=core.MAXAuthSession}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Failure 409 {object} response.ErrorEnvelope
// @Router /auth/max/complete [post]
func (h *AuthHandler) CompleteMAXAuth(w http.ResponseWriter, r *http.Request) {
	var req CompleteMAXAuthRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid json body")
		return
	}

	session, err := h.service.CompleteMAXAuth(r.Context(), req.SessionID, req.InitData)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), authErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, session)
}

// GetMAXAuthSession godoc
// @Summary Get MAX auth session status
// @Tags auth
// @Produce json
// @Param sessionID path string true "Auth session ID"
// @Success 200 {object} response.SuccessEnvelope{data=core.MAXAuthSession}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Router /auth/max/session/{sessionID} [get]
func (h *AuthHandler) GetMAXAuthSession(w http.ResponseWriter, r *http.Request) {
	session, err := h.service.GetMAXAuthSession(r.Context(), chi.URLParam(r, "sessionID"))
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), authErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, session)
}

// ExchangeMAXAuth godoc
// @Summary Exchange completed MAX auth session for backend JWT
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ExchangeMAXAuthRequest true "Auth session exchange payload"
// @Success 200 {object} response.SuccessEnvelope{data=core.AuthSession}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Failure 409 {object} response.ErrorEnvelope
// @Router /auth/max/exchange [post]
func (h *AuthHandler) ExchangeMAXAuth(w http.ResponseWriter, r *http.Request) {
	var req ExchangeMAXAuthRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid json body")
		return
	}

	session, err := h.service.ExchangeMAXAuth(r.Context(), req.SessionID)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), authErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, session)
}

type UpdateMeRequest struct {
	FullName *string `json:"full_name"`
	Phone    *string `json:"phone"`
}

// UpdateMe godoc
// @Summary Update current user profile
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateMeRequest true "Profile update payload"
// @Success 200 {object} response.SuccessEnvelope{data=core.User}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Failure 409 {object} response.ErrorEnvelope
// @Router /me [patch]
func (h *AuthHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Failure(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req UpdateMeRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid json body")
		return
	}

	user, err := h.service.UpdateProfile(r.Context(), userID, service.UpdateProfileInput{
		FullName: req.FullName,
		Phone:    req.Phone,
	})
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), authErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, user)
}

func authErrorMessage(err error) string {
	switch {
	case errors.Is(err, errorsstatus.ErrInvalidInput):
		return "invalid input"
	case errors.Is(err, errorsstatus.ErrUnauthorized):
		switch {
		case errors.Is(err, service.ErrMAXInitDataExpired):
			return "max init data expired"
		case errors.Is(err, service.ErrMAXStartParamMismatch):
			return "max start param mismatch"
		default:
			return "unauthorized"
		}
	case errors.Is(err, errorsstatus.ErrConflict):
		switch {
		case errors.Is(err, service.ErrMAXAuthSessionNotReady):
			return "auth session not ready"
		case errors.Is(err, service.ErrMAXAuthSessionExpired):
			return "auth session expired"
		case errors.Is(err, service.ErrMAXAuthSessionAlreadyExchanged):
			return "auth session already exchanged"
		case errors.Is(err, service.ErrMAXAuthSessionAlreadyUsed):
			return "auth session already completed"
		default:
			return "conflict"
		}
	case errors.Is(err, errorsstatus.ErrNotFound):
		return "not found"
	case errors.Is(err, errorsstatus.ErrServiceUnavailable):
		return "max auth is unavailable"
	default:
		return "internal server error"
	}
}
