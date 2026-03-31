package action

import (
	"encoding/json"
	"errors"
	"net/http"

	"eventAI/internal/adapters/api/middleware"
	"eventAI/internal/adapters/api/response"
	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/service"
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
		return "unauthorized"
	case errors.Is(err, errorsstatus.ErrConflict):
		return "conflict"
	case errors.Is(err, errorsstatus.ErrNotFound):
		return "user not found"
	default:
		return "internal server error"
	}
}
