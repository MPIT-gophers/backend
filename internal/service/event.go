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
	defaultPointSearchEventType = "мероприятие"
	defaultBudgetType           = "total"
	defaultBudgetCurrency       = "RUB"
	defaultVariantSource        = "initial"
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

	pointSearchResponse, err := s.generator.PointSearch(ctx, n8n.PointSearchRequest{
		Event: inferPointSearchEventType(energy),
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
	})
	if err != nil {
		if updateErr := s.repo.FailGeneration(ctx, event.ID, err.Error()); updateErr != nil {
			return core.Event{}, updateErr
		}

		return core.Event{}, fmt.Errorf("%w: %v", errorsstatus.ErrServiceUnavailable, err)
	}

	generatedVariant := buildGeneratedVariant(city, energy, input.Scale, pointSearchTime, pointSearchResponse)
	if err := s.repo.SaveGeneratedVariant(ctx, event.ID, generatedVariant); err != nil {
		if failErr := s.repo.FailGeneration(ctx, event.ID, err.Error()); failErr != nil {
			return core.Event{}, failErr
		}
		return core.Event{}, err
	}

	event.Status = core.EventStatusReady
	event.Title = generatedVariant.Title
	event.Description = generatedVariant.Description
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

func (s *EventService) SelectVariant(ctx context.Context, actorID, eventID, variantID string) (core.Event, error) {
	actorID = strings.TrimSpace(actorID)
	eventID = strings.TrimSpace(eventID)
	variantID = strings.TrimSpace(variantID)

	if actorID == "" || eventID == "" || variantID == "" {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	role, err := s.repo.GetAccessRole(ctx, actorID, eventID)
	if err != nil {
		return core.Event{}, err
	}
	if role != "organizer" {
		return core.Event{}, errorsstatus.ErrForbidden
	}

	event, err := s.repo.SelectVariant(ctx, eventID, variantID)
	if err != nil {
		return core.Event{}, err
	}

	event.AccessRole = &role
	return event, nil
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

func inferPointSearchEventType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return defaultPointSearchEventType
	}

	switch {
	case containsAny(normalized, "день рождения", "деньрождения", "др", "юбилей"):
		return "день рождения"
	case containsAny(normalized, "свадьба", "венчание"):
		return "свадьба"
	case containsAny(normalized, "девичник"):
		return "девичник"
	case containsAny(normalized, "мальчишник"):
		return "мальчишник"
	case containsAny(normalized, "выпускной"):
		return "выпускной"
	case containsAny(normalized, "корпоратив", "тимбилдинг", "тим билдинг", "team building", "команд"):
		return "корпоратив"
	case containsAny(normalized, "конференц", "форум", "митап", "семинар", "презентац"):
		return "деловое мероприятие"
	case containsAny(normalized, "свидание", "романтик"):
		return "свидание"
	default:
		return defaultPointSearchEventType
	}
}

func buildGeneratedVariant(city, energy string, scale int, timeValue string, response n8n.PointSearchResponse) repo.GeneratedEventVariant {
	title := fmt.Sprintf("Подборка площадок в %s", city)

	locations := make([]repo.GeneratedEventLocation, 0, len(response.Venues))
	for i, venue := range response.Venues {
		location := buildGeneratedLocation(city, i, venue)
		if location == nil {
			continue
		}
		locations = append(locations, *location)
	}

	focusLabel := normalizeEventFocus(energy)
	description := fmt.Sprintf(
		"Найдено %d мест для компании %d чел. с фокусом на \"%s\" к %s.",
		len(locations),
		scale,
		focusLabel,
		timeValue,
	)

	return repo.GeneratedEventVariant{
		Title:       &title,
		Description: &description,
		Locations:   locations,
	}
}

func buildGeneratedLocation(city string, index int, venue n8n.PointSearchVenue) *repo.GeneratedEventLocation {
	title := firstNonEmpty(derefString(venue.Name), derefString(venue.AddressName), derefString(venue.Address), fmt.Sprintf("Локация %d", index+1))
	address := firstNonNilString(venue.Address, venue.AddressName)

	if isBogusVenue(city, title, address) {
		return nil
	}

	return &repo.GeneratedEventLocation{
		Title:        title,
		ImageURL:     venue.ImageURL,
		Description:  buildVenueDescription(venue),
		Rating:       nullableFloatString(venue.Rating),
		Address:      address,
		WorkingHours: venue.WorkingHours,
		AvgBill:      venue.AvgBill,
		Cuisine:      venue.Cuisine,
		Contacts:     venue.Contacts,
		AIComment:    nullableString(joinNonEmpty([]string{derefString(venue.PurposeName), derefString(venue.AddressComment), derefString(venue.Type)}, " • ")),
		AIScore:      nil,
		SortOrder:    index + 1,
		Source:       defaultVariantSource,
	}
}

func formatPointSearchTime(value string) (string, error) {
	parsed, err := time.Parse("15:04:05", value)
	if err != nil {
		return "", fmt.Errorf("invalid point search time")
	}

	return parsed.Format("15:04"), nil
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}

	return strings.TrimSpace(*value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func firstNonNilString(values ...*string) *string {
	for _, value := range values {
		if value == nil {
			continue
		}
		trimmed := strings.TrimSpace(*value)
		if trimmed == "" {
			continue
		}
		return &trimmed
	}

	return nil
}

func joinNonEmpty(values []string, separator string) string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}

	return strings.Join(filtered, separator)
}

func nullableString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	return &value
}

func containsAny(value string, substrings ...string) bool {
	for _, substring := range substrings {
		if strings.Contains(value, substring) {
			return true
		}
	}

	return false
}

func buildVenueDescription(venue n8n.PointSearchVenue) *string {
	return nullableString(firstNonEmpty(
		derefString(venue.Description),
		joinNonEmpty([]string{derefString(venue.PurposeName), derefString(venue.AddressComment), derefString(venue.Type)}, " • "),
	))
}

func nullableFloatString(value *float64) *string {
	if value == nil {
		return nil
	}

	formatted := fmt.Sprintf("%.2f", *value)
	return &formatted
}

func normalizeEventFocus(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" || normalized == "0" {
		return "без уточнённого фокуса"
	}

	return normalized
}

func isBogusVenue(city string, title string, address *string) bool {
	title = strings.TrimSpace(strings.ToLower(title))
	city = strings.TrimSpace(strings.ToLower(city))
	if title == "" {
		return true
	}

	if address == nil {
		return title == city
	}

	normalizedAddress := strings.TrimSpace(strings.ToLower(*address))
	return title == city && normalizedAddress == city
}

func GenerateInviteToken() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	return hex.EncodeToString(raw), nil
}
