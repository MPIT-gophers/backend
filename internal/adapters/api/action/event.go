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

type EventHandler struct {
	service *service.EventService
}

func NewEventHandler(service *service.EventService) *EventHandler {
	return &EventHandler{service: service}
}

type CreateEventRequest struct {
	City               string `json:"city"`
	EventDate          string `json:"event_date"`
	EventTime          string `json:"event_time"`
	ExpectedGuestCount int    `json:"expected_guest_count"`
	Budget             string `json:"budget"`
}

// Create godoc
// @Summary Create event
// @Tags events
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateEventRequest true "Event payload"
// @Success 201 {object} response.SuccessEnvelope{data=core.Event}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 409 {object} response.ErrorEnvelope
// @Router /events [post]
func (h *EventHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Failure(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateEventRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid json body")
		return
	}

	event, err := h.service.Create(r.Context(), userID, service.CreateEventInput{
		City:               req.City,
		EventDate:          req.EventDate,
		EventTime:          req.EventTime,
		ExpectedGuestCount: req.ExpectedGuestCount,
		Budget:             req.Budget,
	})
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), eventErrorMessage(err))
		return
	}

	response.Success(w, http.StatusCreated, event)
}

// ListMine godoc
// @Summary List my events
// @Tags events
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.SuccessEnvelope{data=[]core.Event}
// @Failure 401 {object} response.ErrorEnvelope
// @Router /events/my [get]
func (h *EventHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Failure(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	events, err := h.service.ListMine(r.Context(), userID)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), eventErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, events)
}

type JoinEventByTokenRequest struct {
	Token string `json:"token"`
}

// JoinByToken godoc
// @Summary Join event by invite token
// @Tags events
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body JoinEventByTokenRequest true "Join event payload"
// @Success 200 {object} response.SuccessEnvelope{data=core.Event}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Failure 409 {object} response.ErrorEnvelope
// @Router /events/join-by-token [post]
func (h *EventHandler) JoinByToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Failure(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req JoinEventByTokenRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid json body")
		return
	}

	event, err := h.service.JoinByToken(r.Context(), userID, req.Token)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), eventErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, event)
}

// GetByID godoc
// @Summary Get event by id
// @Tags events
// @Produce json
// @Security BearerAuth
// @Param eventID path string true "Event ID"
// @Success 200 {object} response.SuccessEnvelope{data=core.Event}
// @Failure 400 {object} response.ErrorEnvelope
// @Failure 401 {object} response.ErrorEnvelope
// @Failure 403 {object} response.ErrorEnvelope
// @Failure 404 {object} response.ErrorEnvelope
// @Router /events/{eventID} [get]
func (h *EventHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventID")
	event, err := h.service.GetByID(r.Context(), eventID)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), eventErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, event)
}

func eventErrorMessage(err error) string {
	switch {
	case errors.Is(err, errorsstatus.ErrInvalidInput):
		return "invalid input"
	case errors.Is(err, errorsstatus.ErrUnauthorized):
		return "unauthorized"
	case errors.Is(err, errorsstatus.ErrForbidden):
		return "forbidden"
	case errors.Is(err, errorsstatus.ErrConflict):
		return "conflict"
	case errors.Is(err, errorsstatus.ErrNotFound):
		return "not found"
	default:
		return "internal server error"
	}
}
