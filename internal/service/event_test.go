package service

import (
	"context"
	"errors"
	"testing"

	"eventAI/internal/entities/core"
	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/repo"
	"eventAI/pkg/n8n"
)

func TestEventServiceCreateNormalizesInput(t *testing.T) {
	t.Parallel()

	repository := &stubEventRepository{
		createResult: core.Event{ID: "event-1", Status: core.EventStatusGenerating},
	}
	generator := &stubEventGenerator{}
	service := NewEventService(repository, generator)

	event, err := service.Create(context.Background(), " user-1 ", CreateEventInput{
		City:   "  Якутск  ",
		Date:   "2099-06-01",
		Time:   "19:30",
		Scale:  12,
		Energy: "  уютный вайб  ",
		Budget: "15000,5",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if event.ID != "event-1" {
		t.Fatalf("event id = %q, want event-1", event.ID)
	}

	if repository.lastCreateParams.UserID != "user-1" {
		t.Fatalf("user id = %q, want user-1", repository.lastCreateParams.UserID)
	}

	if repository.lastCreateParams.City != "Якутск" {
		t.Fatalf("city = %q, want Якутск", repository.lastCreateParams.City)
	}

	if repository.lastCreateParams.EventTime != "19:30:00" {
		t.Fatalf("event time = %q, want 19:30:00", repository.lastCreateParams.EventTime)
	}

	if repository.lastCreateParams.Budget != "15000.50" {
		t.Fatalf("budget = %q, want 15000.50", repository.lastCreateParams.Budget)
	}

	if generator.lastRequest.Event != "корпоратив" {
		t.Fatalf("event = %q, want корпоратив", generator.lastRequest.Event)
	}

	if generator.lastRequest.Time != "19:30" {
		t.Fatalf("time = %q, want 19:30", generator.lastRequest.Time)
	}

	if generator.lastRequest.Participants != 12 {
		t.Fatalf("participants = %d, want 12", generator.lastRequest.Participants)
	}

	if len(generator.lastRequest.Preferences) != 1 || generator.lastRequest.Preferences[0] != "уютный вайб" {
		t.Fatalf("preferences = %#v, want [\"уютный вайб\"]", generator.lastRequest.Preferences)
	}

	if generator.lastRequest.Budget.Type != "total" {
		t.Fatalf("budget.type = %q, want total", generator.lastRequest.Budget.Type)
	}

	if generator.lastRequest.Budget.Amount != 15000.50 {
		t.Fatalf("budget.amount = %v, want 15000.50", generator.lastRequest.Budget.Amount)
	}
}

func TestEventServiceCreateRejectsPastDate(t *testing.T) {
	t.Parallel()

	service := NewEventService(&stubEventRepository{}, &stubEventGenerator{})

	_, err := service.Create(context.Background(), "user-1", CreateEventInput{
		City:   "Якутск",
		Date:   "2000-01-01",
		Time:   "19:30",
		Scale:  10,
		Energy: "весело",
		Budget: "1000",
	})
	if !errors.Is(err, errorsstatus.ErrInvalidInput) {
		t.Fatalf("error = %v, want invalid input", err)
	}
}

func TestEventServiceCreateRejectsUnauthorizedUser(t *testing.T) {
	t.Parallel()

	service := NewEventService(&stubEventRepository{}, &stubEventGenerator{})

	_, err := service.Create(context.Background(), "   ", CreateEventInput{
		City:   "Якутск",
		Date:   "2099-06-01",
		Time:   "19:30",
		Scale:  10,
		Energy: "весело",
		Budget: "1000",
	})
	if !errors.Is(err, errorsstatus.ErrUnauthorized) {
		t.Fatalf("error = %v, want unauthorized", err)
	}
}

func TestEventServiceCreateMarksEventFailedWhenGeneratorUnavailable(t *testing.T) {
	t.Parallel()

	repository := &stubEventRepository{
		createResult: core.Event{ID: "event-1", Status: core.EventStatusGenerating},
	}
	generator := &stubEventGenerator{
		pointSearchErr: errors.New("n8n is down"),
	}
	service := NewEventService(repository, generator)

	_, err := service.Create(context.Background(), "user-1", CreateEventInput{
		City:   "Якутск",
		Date:   "2099-06-01",
		Time:   "19:30",
		Scale:  10,
		Energy: "караоке",
		Budget: "1000",
	})
	if !errors.Is(err, errorsstatus.ErrServiceUnavailable) {
		t.Fatalf("error = %v, want service unavailable", err)
	}

	if repository.lastUpdateStatusEventID != "event-1" {
		t.Fatalf("updated event id = %q, want event-1", repository.lastUpdateStatusEventID)
	}

	if repository.lastUpdateStatusValue != core.EventStatusFailed {
		t.Fatalf("updated status = %q, want failed", repository.lastUpdateStatusValue)
	}
}

func TestEventServiceJoinByTokenNormalizesToken(t *testing.T) {
	t.Parallel()

	repository := &stubEventRepository{
		joinResult: core.Event{ID: "event-2"},
	}
	service := NewEventService(repository, &stubEventGenerator{})

	_, err := service.JoinByToken(context.Background(), " user-1 ", " token-123 ")
	if err != nil {
		t.Fatalf("JoinByToken() error = %v", err)
	}

	if repository.lastJoinParams.UserID != "user-1" {
		t.Fatalf("user id = %q, want user-1", repository.lastJoinParams.UserID)
	}

	if repository.lastJoinParams.Token != "token-123" {
		t.Fatalf("token = %q, want token-123", repository.lastJoinParams.Token)
	}
}

func TestEventServiceListMineRejectsUnauthorizedUser(t *testing.T) {
	t.Parallel()

	service := NewEventService(&stubEventRepository{}, &stubEventGenerator{})

	_, err := service.ListMine(context.Background(), "")
	if !errors.Is(err, errorsstatus.ErrUnauthorized) {
		t.Fatalf("error = %v, want unauthorized", err)
	}
}

func TestEventServiceGetInviteToken(t *testing.T) {
	t.Parallel()
	repo := &stubEventRepository{
		inviteTokenResult: "invite-123",
	}
	s := NewEventService(repo, &stubEventGenerator{})

	token, err := s.GetInviteToken(context.Background(), "event-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "invite-123" {
		t.Fatalf("got token %q, want invite-123", token)
	}

	_, err = s.GetInviteToken(context.Background(), "  ")
	if !errors.Is(err, errorsstatus.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestEventServiceListGuests(t *testing.T) {
	t.Parallel()
	repo := &stubEventRepository{
		listGuestsResult: []core.EventGuest{{ID: "guest-1"}},
	}
	s := NewEventService(repo, &stubEventGenerator{})

	guests, err := s.ListGuests(context.Background(), "event-1", "approved")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(guests) != 1 || guests[0].ID != "guest-1" {
		t.Fatalf("unexpected guests result: %+v", guests)
	}
	if repo.lastListGuestsFilter == nil || *repo.lastListGuestsFilter != "approved" {
		t.Fatalf("expected filter 'approved'")
	}

	_, err = s.ListGuests(context.Background(), " ", "approved")
	if !errors.Is(err, errorsstatus.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	_, err = s.ListGuests(context.Background(), "event-1", "invalid_status")
	if !errors.Is(err, errorsstatus.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for wrong status, got %v", err)
	}
}

func TestEventServiceUpdateGuestStatus(t *testing.T) {
	t.Parallel()

	t.Run("AttendanceSuccess", func(t *testing.T) {
		repo := &stubEventRepository{
			roleResult:             "guest_approved",
			updateAttendanceResult: core.EventGuest{ID: "guest-1", AttendanceStatus: "confirmed"},
		}
		s := NewEventService(repo, &stubEventGenerator{})

		status := "confirmed"
		guest, err := s.UpdateGuestStatus(context.Background(), "user-1", "event-1", "guest-1", UpdateGuestStatusInput{
			AttendanceStatus: &status,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if guest.AttendanceStatus != "confirmed" {
			t.Fatalf("unexpected attendance: %s", guest.AttendanceStatus)
		}
	})

	t.Run("InvalidInputs", func(t *testing.T) {
		s := NewEventService(&stubEventRepository{}, &stubEventGenerator{})

		_, err := s.UpdateGuestStatus(context.Background(), "user-1", "event-1", "guest-1", UpdateGuestStatusInput{})
		if !errors.Is(err, errorsstatus.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})

	t.Run("ForbiddenRoles", func(t *testing.T) {
		repo := &stubEventRepository{roleResult: "organizer"}
		s := NewEventService(repo, &stubEventGenerator{})
		attStatus := "confirmed"
		_, err := s.UpdateGuestStatus(context.Background(), "user-1", "event-1", "guest-1", UpdateGuestStatusInput{
			AttendanceStatus: &attStatus,
		})
		if !errors.Is(err, errorsstatus.ErrForbidden) {
			t.Fatalf("expected ErrForbidden for organizer setting attendance, got %v", err)
		}
	})
}

type stubEventRepository struct {
	createResult     core.Event
	createErr        error
	lastCreateParams repo.CreateEventParams
	updateStatusErr  error

	lastUpdateStatusEventID string
	lastUpdateStatusValue   string

	listResult     []core.Event
	listErr        error
	lastListUserID string

	joinResult     core.Event
	joinErr        error
	lastJoinParams repo.JoinEventByTokenParams

	getResult      core.Event
	getErr         error
	lastGetEventID string

	roleResult      string
	roleErr         error
	lastRoleUserID  string
	lastRoleEventID string

	inviteTokenResult      string
	inviteTokenErr         error
	lastInviteTokenEventID string

	listGuestsResult     []core.EventGuest
	listGuestsErr        error
	lastListGuestsFilter *string

	updateAttendanceResult     core.EventGuest
	updateAttendanceErr        error
	lastUpdateAttendanceParams repo.UpdateGuestAttendanceParams

	guestStatsResult core.EventGuestStats
	guestStatsErr    error
}

func (s *stubEventRepository) Create(_ context.Context, params repo.CreateEventParams) (core.Event, error) {
	s.lastCreateParams = params
	return s.createResult, s.createErr
}

func (s *stubEventRepository) UpdateStatus(_ context.Context, eventID string, status string) error {
	s.lastUpdateStatusEventID = eventID
	s.lastUpdateStatusValue = status
	return s.updateStatusErr
}

func (s *stubEventRepository) ListMine(_ context.Context, userID string) ([]core.Event, error) {
	s.lastListUserID = userID
	return s.listResult, s.listErr
}

func (s *stubEventRepository) JoinByToken(_ context.Context, params repo.JoinEventByTokenParams) (core.Event, error) {
	s.lastJoinParams = params
	return s.joinResult, s.joinErr
}

func (s *stubEventRepository) GetByID(_ context.Context, eventID string) (core.Event, error) {
	s.lastGetEventID = eventID
	return s.getResult, s.getErr
}

func (s *stubEventRepository) GetAccessRole(_ context.Context, userID string, eventID string) (string, error) {
	s.lastRoleUserID = userID
	s.lastRoleEventID = eventID
	return s.roleResult, s.roleErr
}

func (s *stubEventRepository) GetInviteToken(_ context.Context, eventID string) (string, error) {
	s.lastInviteTokenEventID = eventID
	return s.inviteTokenResult, s.inviteTokenErr
}

func (s *stubEventRepository) ListGuests(_ context.Context, eventID string, filter *string) ([]core.EventGuest, error) {
	s.lastListGuestsFilter = filter
	return s.listGuestsResult, s.listGuestsErr
}

func (s *stubEventRepository) UpdateGuestAttendanceStatus(_ context.Context, params repo.UpdateGuestAttendanceParams) (core.EventGuest, error) {
	s.lastUpdateAttendanceParams = params
	return s.updateAttendanceResult, s.updateAttendanceErr
}

func (s *stubEventRepository) GetGuestStats(_ context.Context, eventID string) (core.EventGuestStats, error) {
	return s.guestStatsResult, s.guestStatsErr
}

func (s *stubEventRepository) GetGuestAttendanceStatus(_ context.Context, eventID string, userID string) (core.AttendanceStatus, error) {
	return "", nil
}

type stubEventGenerator struct {
	pointSearchResult n8n.PointSearchResponse
	pointSearchErr    error
	lastRequest       n8n.PointSearchRequest
}

func (s *stubEventGenerator) PointSearch(_ context.Context, input n8n.PointSearchRequest) (n8n.PointSearchResponse, error) {
	s.lastRequest = input
	return s.pointSearchResult, s.pointSearchErr
}
