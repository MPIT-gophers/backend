package action

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"eventAI/internal/adapters/api/middleware"
	"eventAI/internal/entities/core"
	"eventAI/internal/repo"
	"eventAI/internal/service"

	"github.com/go-chi/chi/v5"
)

type eventResponse struct {
	Data core.Event `json:"data"`
}

func TestEventHandlerCreateSuccess(t *testing.T) {
	t.Parallel()

	handler := NewEventHandler(service.NewEventService(&stubEventRepository{
		createResult: core.Event{
			ID:                "event-1",
			City:              "Якутск",
			EventDate:         "2099-06-01",
			EventTime:         "19:30:00",
			Status:            "draft",
			SelectedVariantID: nil,
		},
	}))

	wrapped := middleware.Auth(func(token string) (string, error) {
		return "user-1", nil
	})(http.HandlerFunc(handler.Create))

	body := bytes.NewBufferString(`{"city":"Якутск","event_date":"2099-06-01","event_time":"19:30","expected_guest_count":10,"budget":"15000"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/events", body)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var response eventResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.ID != "event-1" {
		t.Fatalf("event id = %q, want event-1", response.Data.ID)
	}
}

func TestEventHandlerCreateInvalidJSON(t *testing.T) {
	t.Parallel()

	handler := NewEventHandler(service.NewEventService(&stubEventRepository{}))
	wrapped := middleware.Auth(func(token string) (string, error) {
		return "user-1", nil
	})(http.HandlerFunc(handler.Create))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewBufferString(`{"city":`))
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestEventHandlerGetByIDSuccess(t *testing.T) {
	t.Parallel()

	handler := NewEventHandler(service.NewEventService(&stubEventRepository{
		getResult: core.Event{
			ID:                "event-42",
			City:              "Якутск",
			EventDate:         "2099-06-01",
			EventTime:         "19:30:00",
			Status:            "draft",
			SelectedVariantID: strPtr("variant-1"),
			Variants: []core.EventVariant{
				{
					ID:            "variant-1",
					EventID:       "event-42",
					VariantNumber: 1,
					Status:        "ready",
				},
			},
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/event-42", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("eventID", "event-42")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.GetByID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var response eventResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Data.SelectedVariantID == nil || *response.Data.SelectedVariantID != "variant-1" {
		t.Fatalf("selected variant id = %v, want variant-1", response.Data.SelectedVariantID)
	}

	if len(response.Data.Variants) != 1 {
		t.Fatalf("variants len = %d, want 1", len(response.Data.Variants))
	}
}

type stubEventRepository struct {
	createResult core.Event
	listResult   []core.Event
	joinResult   core.Event
	getResult    core.Event
	roleResult   string

	inviteTokenResult      string
	inviteTokenErr         error
	listGuestsResult      []core.EventGuest
	listGuestsErr         error
	updateApprovalResult      core.EventGuest
	updateApprovalErr         error
	updateAttendanceResult     core.EventGuest
	updateAttendanceErr        error
	guestStatsResult      core.EventGuestStats
	guestStatsErr         error
}

func (s *stubEventRepository) Create(_ context.Context, _ repo.CreateEventParams) (core.Event, error) {
	return s.createResult, nil
}

func (s *stubEventRepository) ListMine(_ context.Context, _ string) ([]core.Event, error) {
	return s.listResult, nil
}

func (s *stubEventRepository) JoinByToken(_ context.Context, _ repo.JoinEventByTokenParams) (core.Event, error) {
	return s.joinResult, nil
}

func (s *stubEventRepository) GetByID(_ context.Context, _ string) (core.Event, error) {
	return s.getResult, nil
}

func (s *stubEventRepository) GetAccessRole(_ context.Context, _ string, _ string) (string, error) {
	return s.roleResult, nil
}

func (s *stubEventRepository) GetInviteToken(_ context.Context, _ string) (string, error) {
	return s.inviteTokenResult, s.inviteTokenErr
}

func (s *stubEventRepository) ListGuests(_ context.Context, _ string, _ *string) ([]core.EventGuest, error) {
	return s.listGuestsResult, s.listGuestsErr
}

func (s *stubEventRepository) UpdateGuestApprovalStatus(_ context.Context, _ repo.UpdateGuestApprovalParams) (core.EventGuest, error) {
	return s.updateApprovalResult, s.updateApprovalErr
}

func (s *stubEventRepository) UpdateGuestAttendanceStatus(_ context.Context, _ repo.UpdateGuestAttendanceParams) (core.EventGuest, error) {
	return s.updateAttendanceResult, s.updateAttendanceErr
}

func (s *stubEventRepository) GetGuestStats(_ context.Context, _ string) (core.EventGuestStats, error) {
	return s.guestStatsResult, s.guestStatsErr
}

func TestEventHandlerGetInviteSuccess(t *testing.T) {
	t.Parallel()

	handler := NewEventHandler(service.NewEventService(&stubEventRepository{
		roleResult: "organizer",
		inviteTokenResult: "invite-123",
	}))

	wrapped := middleware.Auth(func(token string) (string, error) {
		return "user-1", nil
	})(http.HandlerFunc(handler.GetInvite))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/event-1/invite", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("eventID", "event-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestEventHandlerJoinByTokenSuccess(t *testing.T) {
	t.Parallel()

	handler := NewEventHandler(service.NewEventService(&stubEventRepository{
		joinResult: core.Event{ID: "event-2", Status: "published"},
	}))

	wrapped := middleware.Auth(func(token string) (string, error) {
		return "user-2", nil
	})(http.HandlerFunc(handler.JoinByToken))

	body := bytes.NewBufferString(`{"token":"invite-123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/events/join-by-token", body)
	req.Header.Set("Authorization", "Bearer good-token")

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestEventHandlerUpdateGuestStatusSuccess(t *testing.T) {
	t.Parallel()

	handler := NewEventHandler(service.NewEventService(&stubEventRepository{
		roleResult: "organizer",
		updateApprovalResult: core.EventGuest{ID: "guest-1", ApprovalStatus: "approved"},
	}))

	wrapped := middleware.Auth(func(token string) (string, error) {
		return "user-1", nil
	})(http.HandlerFunc(handler.UpdateGuestStatus))

	body := bytes.NewBufferString(`{"approval_status":"approved"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/events/event-1/guests/guest-1/status", body)
	req.Header.Set("Authorization", "Bearer good-token")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("eventID", "event-1")
	rctx.URLParams.Add("guestID", "guest-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
