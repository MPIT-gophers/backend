package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"eventAI/internal/entities/core"
	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/repo"
)

type EventService struct {
	repo repo.EventRepository
}

func NewEventService(repo repo.EventRepository) *EventService {
	return &EventService{
		repo: repo,
	}
}

type CreateEventInput struct {
	City               string
	EventDate          string
	EventTime          string
	ExpectedGuestCount int
	Budget             string
}

func (s *EventService) Create(ctx context.Context, userID string, input CreateEventInput) (core.Event, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return core.Event{}, errorsstatus.ErrUnauthorized
	}

	city := strings.TrimSpace(input.City)
	if len([]rune(city)) < 2 || len([]rune(city)) > 255 {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	eventDate, err := time.Parse("2006-01-02", strings.TrimSpace(input.EventDate))
	if err != nil {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	if eventDate.Before(today) {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	eventTime, err := normalizeEventTime(input.EventTime)
	if err != nil {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	if input.ExpectedGuestCount <= 0 {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	budget, err := normalizeBudget(input.Budget)
	if err != nil {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	return s.repo.Create(ctx, repo.CreateEventParams{
		UserID:             userID,
		City:               city,
		EventDate:          eventDate.Format("2006-01-02"),
		EventTime:          eventTime,
		ExpectedGuestCount: input.ExpectedGuestCount,
		Budget:             budget,
	})
}

func (s *EventService) ListMine(ctx context.Context, userID string) ([]core.Event, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errorsstatus.ErrUnauthorized
	}

	return s.repo.ListMine(ctx, userID)
}

func (s *EventService) JoinByToken(ctx context.Context, userID string, token string) (core.Event, error) {
	userID = strings.TrimSpace(userID)
	token = strings.TrimSpace(token)
	if userID == "" || token == "" {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	return s.repo.JoinByToken(ctx, repo.JoinEventByTokenParams{
		UserID: userID,
		Token:  token,
	})
}

func (s *EventService) GetByID(ctx context.Context, eventID string) (core.Event, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	return s.repo.GetByID(ctx, eventID)
}

type UpdateGuestStatusInput struct {
	AttendanceStatus *string
}

func (s *EventService) GetInviteToken(ctx context.Context, eventID string) (string, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return "", errorsstatus.ErrInvalidInput
	}

	return s.repo.GetInviteToken(ctx, eventID)
}

func (s *EventService) ListGuests(ctx context.Context, eventID string, approvalStatus string) ([]core.EventGuest, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return nil, errorsstatus.ErrInvalidInput
	}

	var filter *string
	if v := strings.TrimSpace(approvalStatus); v != "" {
		switch core.ApprovalStatus(v) {
		case core.ApprovalPending, core.ApprovalApproved, core.ApprovalRejected:
		default:
			return nil, errorsstatus.ErrInvalidInput
		}
		filter = &v
	}

	return s.repo.ListGuests(ctx, eventID, filter)
}

func (s *EventService) UpdateGuestStatus(ctx context.Context, actorID, eventID, guestID string, input UpdateGuestStatusInput) (core.EventGuest, error) {
	actorID = strings.TrimSpace(actorID)
	eventID = strings.TrimSpace(eventID)
	guestID = strings.TrimSpace(guestID)

	if actorID == "" || eventID == "" || guestID == "" {
		return core.EventGuest{}, errorsstatus.ErrInvalidInput
	}

	if input.AttendanceStatus == nil {
		return core.EventGuest{}, errorsstatus.ErrInvalidInput
	}

	role, err := s.repo.GetAccessRole(ctx, actorID, eventID)
	if err != nil {
		return core.EventGuest{}, err
	}

	// attendance_status — только гость
	if role == "organizer" || role == "co_host" {
		return core.EventGuest{}, errorsstatus.ErrForbidden
	}
	status := core.AttendanceStatus(strings.TrimSpace(*input.AttendanceStatus))
	if status != core.AttendanceConfirmed && status != core.AttendanceDeclined {
		return core.EventGuest{}, errorsstatus.ErrInvalidInput
	}
	return s.repo.UpdateGuestAttendanceStatus(ctx, repo.UpdateGuestAttendanceParams{
		GuestID:          guestID,
		EventID:          eventID,
		AttendanceStatus: status,
	})
}

func (s *EventService) GetGuestStats(ctx context.Context, eventID string) (core.EventGuestStats, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return core.EventGuestStats{}, errorsstatus.ErrInvalidInput
	}

	return s.repo.GetGuestStats(ctx, eventID)
}

func normalizeEventTime(value string) (string, error) {
	value = strings.TrimSpace(value)
	layouts := []string{"15:04", "15:04:05"}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.Format("15:04:05"), nil
		}
	}

	return "", fmt.Errorf("invalid event time")
}

func normalizeBudget(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", "."))
	if value == "" {
		return "", fmt.Errorf("empty budget")
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return "", fmt.Errorf("invalid budget")
	}

	return fmt.Sprintf("%.2f", parsed), nil
}

func GenerateInviteToken() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	return hex.EncodeToString(raw), nil
}
