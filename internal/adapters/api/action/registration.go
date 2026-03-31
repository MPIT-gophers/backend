package action

import (
	"encoding/json"
	"errors"
	"net/http"

	"eventAI/internal/adapters/api/response"
	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/usecase"
)

type RegistrationHandler struct {
	useCase *usecase.RegistrationUseCase
}

func NewRegistrationHandler(useCase *usecase.RegistrationUseCase) *RegistrationHandler {
	return &RegistrationHandler{useCase: useCase}
}

type createRegistrationRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}

func (h *RegistrationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRegistrationRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}

	registration, err := h.useCase.Create(r.Context(), usecase.CreateRegistrationInput{
		FullName: req.FullName,
		Email:    req.Email,
	})
	if err != nil {
		response.Error(w, errorsstatus.HTTPStatus(err), errorMessage(err))
		return
	}

	response.JSON(w, http.StatusCreated, "data", registration)
}

func (h *RegistrationHandler) List(w http.ResponseWriter, r *http.Request) {
	registrations, err := h.useCase.List(r.Context())
	if err != nil {
		response.Error(w, errorsstatus.HTTPStatus(err), errorMessage(err))
		return
	}

	response.JSON(w, http.StatusOK, "data", registrations)
}

func errorMessage(err error) string {
	switch {
	case errors.Is(err, errorsstatus.ErrInvalidInput):
		return "invalid input"
	case errors.Is(err, errorsstatus.ErrConflict):
		return "registration already exists"
	case errors.Is(err, errorsstatus.ErrNotFound):
		return "resource not found"
	default:
		return "internal server error"
	}
}
