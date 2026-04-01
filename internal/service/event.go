package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"eventAI/internal/entities/core"
	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/repo"
	"eventAI/pkg/n8n"
)

const (
	defaultPointSearchEventType = "корпоратив"
	defaultBudgetType           = "total"
	defaultBudgetCurrency       = "RUB"
)

type EventGenerator interface {
	PointSearch(ctx context.Context, input n8n.PointSearchRequest) (n8n.PointSearchResponse, error)
}

type EventService struct {
	repo      repo.EventRepository
	generator EventGenerator
}

func NewEventService(repo repo.EventRepository, generator EventGenerator) *EventService {
	return &EventService{
		repo:      repo,
		generator: generator,
	}
}

type CreateEventInput struct {
	City   string
	Date   string
	Time   string
	Scale  int
	Energy string
	Budget string
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

	eventDate, err := time.Parse("2006-01-02", strings.TrimSpace(input.Date))
	if err != nil {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	if eventDate.Before(today) {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	eventTime, err := normalizeEventTime(input.Time)
	if err != nil {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	if input.Scale <= 0 {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	energy := strings.TrimSpace(input.Energy)
	if energy == "" || len([]rune(energy)) > 255 {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	budget, err := normalizeBudget(input.Budget)
	if err != nil {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	budgetAmount, err := strconv.ParseFloat(budget, 64)
	if err != nil || math.IsNaN(budgetAmount) || math.IsInf(budgetAmount, 0) {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	if s.generator == nil {
		return core.Event{}, fmt.Errorf("event generator is not configured")
	}

	event, err := s.repo.Create(ctx, repo.CreateEventParams{
		UserID:             userID,
		City:               city,
		EventDate:          eventDate.Format("2006-01-02"),
		EventTime:          eventTime,
		ExpectedGuestCount: input.Scale,
		Budget:             budget,
	})
	if err != nil {
		return core.Event{}, err
	}

	pointSearchTime, err := formatPointSearchTime(eventTime)
	if err != nil {
		return core.Event{}, err
	}

	if _, err := s.generator.PointSearch(ctx, n8n.PointSearchRequest{
		Event: defaultPointSearchEventType,
		City:  city,
		Date:  eventDate.Format("2006-01-02"),
		Time:  pointSearchTime,
		Budget: n8n.PointSearchBudget{
			Type:     defaultBudgetType,
			Amount:   budgetAmount,
			Currency: defaultBudgetCurrency,
		},
		Participants: input.Scale,
		Preferences:  []string{energy},
	}); err != nil {
		if updateErr := s.repo.UpdateStatus(ctx, event.ID, core.EventStatusFailed); updateErr != nil {
			return core.Event{}, updateErr
		}

		return core.Event{}, fmt.Errorf("%w: %v", errorsstatus.ErrServiceUnavailable, err)
	}

	return event, nil
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

func formatPointSearchTime(value string) (string, error) {
	parsed, err := time.Parse("15:04:05", value)
	if err != nil {
		return "", fmt.Errorf("invalid point search time")
	}

	return parsed.Format("15:04"), nil
}

func GenerateInviteToken() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	return hex.EncodeToString(raw), nil
}
