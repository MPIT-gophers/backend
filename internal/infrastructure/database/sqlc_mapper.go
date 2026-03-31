package database

import (
	"fmt"
	"time"

	"eventAI/internal/entities/core"
	dbsqlc "eventAI/internal/infrastructure/database/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

func parseUUID(value string) (pgtype.UUID, error) {
	var out pgtype.UUID
	if err := out.Scan(value); err != nil {
		return pgtype.UUID{}, err
	}
	return out, nil
}

func parseDate(value string) (pgtype.Date, error) {
	var out pgtype.Date
	if err := out.Scan(value); err != nil {
		return pgtype.Date{}, err
	}
	return out, nil
}

func parseTime(value string) (pgtype.Time, error) {
	var out pgtype.Time
	if err := out.Scan(value); err != nil {
		return pgtype.Time{}, err
	}
	return out, nil
}

func parseNumeric(value string) (pgtype.Numeric, error) {
	var out pgtype.Numeric
	if err := out.Scan(value); err != nil {
		return pgtype.Numeric{}, err
	}
	return out, nil
}

func mapSQLCUser(user dbsqlc.User) core.User {
	return core.User{
		ID:        user.ID.String(),
		FullName:  user.FullName,
		Phone:     user.Phone,
		CreatedAt: timestamptzToTime(user.CreatedAt),
		UpdatedAt: timestamptzToTime(user.UpdatedAt),
	}
}

func mapCreateEventRow(row dbsqlc.CreateEventRow) (core.Event, error) {
	return mapEventBase(
		row.ID,
		row.City,
		row.EventDate,
		row.EventTime,
		row.ExpectedGuestCount,
		row.Budget,
		row.Title,
		row.Description,
		row.Status,
		row.SelectedVariantID,
		row.CreatedAt,
		row.UpdatedAt,
	)
}

func mapGetEventByIDRow(row dbsqlc.GetEventByIDRow) (core.Event, error) {
	return mapEventBase(
		row.ID,
		row.City,
		row.EventDate,
		row.EventTime,
		row.ExpectedGuestCount,
		row.Budget,
		row.Title,
		row.Description,
		row.Status,
		row.SelectedVariantID,
		row.CreatedAt,
		row.UpdatedAt,
	)
}

func mapListMyEventsRow(row dbsqlc.ListMyEventsRow) (core.Event, error) {
	event, err := mapEventBase(
		row.ID,
		row.City,
		row.EventDate,
		row.EventTime,
		row.ExpectedGuestCount,
		row.Budget,
		row.Title,
		row.Description,
		row.Status,
		row.SelectedVariantID,
		row.CreatedAt,
		row.UpdatedAt,
	)
	if err != nil {
		return core.Event{}, err
	}

	event.AccessRole = &row.AccessRole
	event.ApprovalStatus = row.ApprovalStatus
	event.AttendanceStatus = row.AttendanceStatus

	return event, nil
}

func mapGuestAccessEventRow(row dbsqlc.GetEventWithGuestAccessRow) (core.Event, error) {
	event, err := mapEventBase(
		row.ID,
		row.City,
		row.EventDate,
		row.EventTime,
		row.ExpectedGuestCount,
		row.Budget,
		row.Title,
		row.Description,
		row.Status,
		row.SelectedVariantID,
		row.CreatedAt,
		row.UpdatedAt,
	)
	if err != nil {
		return core.Event{}, err
	}

	event.AccessRole = &row.AccessRole
	event.ApprovalStatus = stringPtr(row.EgApprovalStatus)
	event.AttendanceStatus = stringPtr(row.EgAttendanceStatus)

	return event, nil
}

func mapEventBase(
	id pgtype.UUID,
	city string,
	eventDate pgtype.Date,
	eventTime pgtype.Time,
	expectedGuestCount int32,
	budget pgtype.Numeric,
	title *string,
	description *string,
	status string,
	selectedVariantID pgtype.UUID,
	createdAt pgtype.Timestamptz,
	updatedAt pgtype.Timestamptz,
) (core.Event, error) {
	budgetText, err := numericToString(budget)
	if err != nil {
		return core.Event{}, err
	}

	return core.Event{
		ID:                 id.String(),
		City:               city,
		EventDate:          dateToString(eventDate),
		EventTime:          timeToString(eventTime),
		ExpectedGuestCount: int(expectedGuestCount),
		Budget:             budgetText,
		Title:              title,
		Description:        description,
		Status:             status,
		SelectedVariantID:  uuidPtr(selectedVariantID),
		CreatedAt:          timestamptzToTime(createdAt),
		UpdatedAt:          timestamptzToTime(updatedAt),
	}, nil
}

func mapSQLCVariant(variant dbsqlc.EventVariant) core.EventVariant {
	return core.EventVariant{
		ID:              variant.ID.String(),
		EventID:         variant.EventID.String(),
		VariantNumber:   int(variant.VariantNumber),
		Title:           variant.Title,
		Description:     variant.Description,
		Status:          variant.Status,
		LLMRequestID:    variant.LlmRequestID,
		GenerationError: variant.GenerationError,
		CreatedAt:       timestamptzToTime(variant.CreatedAt),
		UpdatedAt:       timestamptzToTime(variant.UpdatedAt),
	}
}

func mapSQLCLocation(location dbsqlc.EventLocation) (core.EventLocation, error) {
	aiScore, err := nullableNumericToString(location.AiScore)
	if err != nil {
		return core.EventLocation{}, err
	}

	return core.EventLocation{
		ID:         location.ID.String(),
		EventID:    location.EventID.String(),
		VariantID:  location.VariantID.String(),
		Title:      location.Title,
		Address:    location.Address,
		Contacts:   location.Contacts,
		AIComment:  location.AiComment,
		AIScore:    aiScore,
		SortOrder:  int(location.SortOrder),
		Source:     location.Source,
		IsRejected: location.IsRejected,
		RejectedAt: timestamptzPtr(location.RejectedAt),
		CreatedAt:  timestamptzToTime(location.CreatedAt),
		UpdatedAt:  timestamptzToTime(location.UpdatedAt),
	}, nil
}

func numericToString(value pgtype.Numeric) (string, error) {
	raw, err := value.Value()
	if err != nil {
		return "", err
	}

	text, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("unexpected numeric value type %T", raw)
	}

	return text, nil
}

func nullableNumericToString(value pgtype.Numeric) (*string, error) {
	if !value.Valid {
		return nil, nil
	}

	text, err := numericToString(value)
	if err != nil {
		return nil, err
	}

	return &text, nil
}

func dateToString(value pgtype.Date) string {
	if !value.Valid {
		return ""
	}

	return value.Time.Format("2006-01-02")
}

func timeToString(value pgtype.Time) string {
	if !value.Valid {
		return ""
	}

	raw, err := value.Value()
	if err != nil {
		return ""
	}

	text, ok := raw.(string)
	if !ok {
		return ""
	}

	if len(text) >= len("15:04:05") {
		return text[:8]
	}

	return text
}

func timestamptzToTime(value pgtype.Timestamptz) time.Time {
	if !value.Valid {
		return time.Time{}
	}

	return value.Time
}

func timestamptzPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}

	out := value.Time
	return &out
}

func uuidPtr(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}

	out := value.String()
	return &out
}

func stringPtr(value string) *string {
	return &value
}
