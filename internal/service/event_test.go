package service

import (
	"context"
	"errors"
	"testing"

	"eventAI/internal/entities/core"
	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/repo"
)

func TestEventServiceCreateNormalizesInput(t *testing.T) {
	t.Parallel()

	repository := &stubEventRepository{
		createResult: core.Event{ID: "event-1"},
	}
	service := NewEventService(repository)

	event, err := service.Create(context.Background(), " user-1 ", CreateEventInput{
		City:               "  Якутск  ",
		EventDate:          "2099-06-01",
		EventTime:          "19:30",
		ExpectedGuestCount: 12,
		Budget:             "15000,5",
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
}

func TestEventServiceCreateRejectsPastDate(t *testing.T) {
	t.Parallel()

	service := NewEventService(&stubEventRepository{})

	_, err := service.Create(context.Background(), "user-1", CreateEventInput{
		City:               "Якутск",
		EventDate:          "2000-01-01",
		EventTime:          "19:30",
		ExpectedGuestCount: 10,
		Budget:             "1000",
	})
	if !errors.Is(err, errorsstatus.ErrInvalidInput) {
		t.Fatalf("error = %v, want invalid input", err)
	}
}

func TestEventServiceCreateRejectsUnauthorizedUser(t *testing.T) {
	t.Parallel()

	service := NewEventService(&stubEventRepository{})

	_, err := service.Create(context.Background(), "   ", CreateEventInput{
		City:               "Якутск",
		EventDate:          "2099-06-01",
		EventTime:          "19:30",
		ExpectedGuestCount: 10,
		Budget:             "1000",
	})
	if !errors.Is(err, errorsstatus.ErrUnauthorized) {
		t.Fatalf("error = %v, want unauthorized", err)
	}
}

func TestEventServiceJoinByTokenNormalizesToken(t *testing.T) {
	t.Parallel()

	repository := &stubEventRepository{
		joinResult: core.Event{ID: "event-2"},
	}
	service := NewEventService(repository)

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

	service := NewEventService(&stubEventRepository{})

	_, err := service.ListMine(context.Background(), "")
	if !errors.Is(err, errorsstatus.ErrUnauthorized) {
		t.Fatalf("error = %v, want unauthorized", err)
	}
}

type stubEventRepository struct {
	createResult     core.Event
	createErr        error
	lastCreateParams repo.CreateEventParams

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
}

func (s *stubEventRepository) Create(_ context.Context, params repo.CreateEventParams) (core.Event, error) {
	s.lastCreateParams = params
	return s.createResult, s.createErr
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
