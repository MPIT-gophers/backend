package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"eventAI/internal/entities/core"
	errorsstatus "eventAI/internal/errorsStatus"
	dbsqlc "eventAI/internal/infrastructure/database/sqlc"
	"eventAI/internal/repo"
	"eventAI/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventRepository struct {
	db      *pgxpool.Pool
	queries *dbsqlc.Queries
}

const createEventSQL = `
INSERT INTO events (
    city,
    event_date,
    event_time,
    expected_guest_count,
    budget,
    status,
    generation_started_at
) VALUES (
    $1,
    $2::date,
    $3::time,
    $4,
    $5::numeric,
    $6,
    NOW()
)
RETURNING
    id,
    city,
    event_date,
    event_time,
    expected_guest_count,
    budget,
    title,
    description,
    status,
    selected_variant_id,
    created_at,
    updated_at
`

const updateEventStatusSQL = `
UPDATE events
SET
    status = $2,
    updated_at = NOW()
WHERE id = $1
`

const failEventGenerationSQL = `
UPDATE events
SET
    status = 'failed',
    generation_finished_at = NOW(),
    generation_error = $2,
    updated_at = NOW()
WHERE id = $1
`

const nextVariantNumberSQL = `
SELECT COALESCE(MAX(variant_number), 0) + 1
FROM event_variants
WHERE event_id = $1
`

const insertEventVariantSQL = `
INSERT INTO event_variants (
    event_id,
    variant_number,
    title,
    description,
    status
) VALUES (
    $1,
    $2,
    $3,
    $4,
    'ready'
)
RETURNING id
`

const insertEventLocationSQL = `
INSERT INTO event_locations (
    event_id,
    variant_id,
    title,
    address,
    contacts,
    ai_comment,
    ai_score,
    sort_order,
    source
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9
)
`

const completeEventGenerationSQL = `
UPDATE events
SET
    status = 'ready',
    generation_finished_at = NOW(),
    generation_error = NULL,
    title = COALESCE($2, title),
    description = COALESCE($3, description),
    updated_at = NOW()
WHERE id = $1
`

const eventLocationExistsSQL = `
SELECT EXISTS(
    SELECT 1
    FROM event_locations
    WHERE event_id = $1
      AND lower(title) = lower($2)
      AND COALESCE(lower(address), '') = COALESCE(lower($3), '')
)
`

const selectEventVariantSQL = `
UPDATE events
SET
    selected_variant_id = $2,
    updated_at = NOW()
WHERE id = $1
  AND EXISTS (
      SELECT 1
      FROM event_variants
      WHERE id = $2
        AND event_id = $1
        AND status = 'ready'
  )
`

func NewEventRepository(db *pgxpool.Pool) repo.EventRepository {
	return &EventRepository{
		db:      db,
		queries: dbsqlc.New(db),
	}
}

func (r *EventRepository) Create(ctx context.Context, params repo.CreateEventParams) (core.Event, error) {
	userID, err := parseUUID(params.UserID)
	if err != nil {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	eventDate, err := parseDate(params.EventDate)
	if err != nil {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	eventTime, err := parseTime(params.EventTime)
	if err != nil {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	budget, err := parseNumeric(params.Budget)
	if err != nil {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return core.Event{}, err
	}
	defer tx.Rollback(ctx)

	queries := r.queries.WithTx(tx)

	var row dbsqlc.CreateEventRow
	err = tx.QueryRow(
		ctx,
		createEventSQL,
		params.City,
		eventDate,
		eventTime,
		int32(params.ExpectedGuestCount),
		budget,
		core.EventStatusGenerating,
	).Scan(
		&row.ID,
		&row.City,
		&row.EventDate,
		&row.EventTime,
		&row.ExpectedGuestCount,
		&row.Budget,
		&row.Title,
		&row.Description,
		&row.Status,
		&row.SelectedVariantID,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		return core.Event{}, mapEventPgError(err)
	}

	event, err := mapCreateEventRow(row)
	if err != nil {
		return core.Event{}, err
	}

	if err := queries.AddEventUser(ctx, dbsqlc.AddEventUserParams{
		EventID: row.ID,
		UserID:  userID,
	}); err != nil {
		return core.Event{}, mapEventPgError(err)
	}

	token, err := service.GenerateInviteToken()
	if err != nil {
		return core.Event{}, err
	}

	if err := queries.CreateEventInvite(ctx, dbsqlc.CreateEventInviteParams{
		EventID:         row.ID,
		CreatedByUserID: userID,
		Token:           token,
	}); err != nil {
		return core.Event{}, mapEventPgError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return core.Event{}, err
	}

	accessRole := "organizer"
	event.AccessRole = &accessRole
	event.InviteToken = &token

	return event, nil
}

func (r *EventRepository) UpdateStatus(ctx context.Context, eventID string, status string) error {
	id, err := parseUUID(eventID)
	if err != nil {
		return errorsstatus.ErrInvalidInput
	}

	tag, err := r.db.Exec(ctx, updateEventStatusSQL, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errorsstatus.ErrNotFound
	}

	return nil
}

func (r *EventRepository) SaveGeneratedVariant(ctx context.Context, eventID string, variant repo.GeneratedEventVariant) error {
	id, err := parseUUID(eventID)
	if err != nil {
		return errorsstatus.ErrInvalidInput
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var variantNumber int32
	if err := tx.QueryRow(ctx, nextVariantNumberSQL, id).Scan(&variantNumber); err != nil {
		return err
	}

	var variantID pgtype.UUID
	if err := tx.QueryRow(ctx, insertEventVariantSQL, id, variantNumber, variant.Title, variant.Description).Scan(&variantID); err != nil {
		return mapEventPgError(err)
	}

	insertedLocations := 0
	seenLocations := make(map[string]struct{}, len(variant.Locations))
	for _, location := range variant.Locations {
		locationKey := buildLocationDedupKey(location.Title, location.Address)
		if _, exists := seenLocations[locationKey]; exists {
			continue
		}
		seenLocations[locationKey] = struct{}{}

		var addressValue *string
		if location.Address != nil {
			trimmed := strings.TrimSpace(*location.Address)
			if trimmed != "" {
				addressValue = &trimmed
			}
		}

		var alreadyExists bool
		if err := tx.QueryRow(ctx, eventLocationExistsSQL, id, location.Title, addressValue).Scan(&alreadyExists); err != nil {
			return err
		}
		if alreadyExists {
			continue
		}

		var aiScore pgtype.Numeric
		if location.AIScore != nil {
			aiScore, err = parseNumeric(*location.AIScore)
			if err != nil {
				return errorsstatus.ErrInvalidInput
			}
		}

		if _, err := tx.Exec(
			ctx,
			insertEventLocationSQL,
			id,
			variantID,
			location.Title,
			addressValue,
			location.Contacts,
			location.AIComment,
			aiScore,
			int32(location.SortOrder),
			location.Source,
		); err != nil {
			return mapEventPgError(err)
		}

		insertedLocations++
	}

	if insertedLocations == 0 {
		return fmt.Errorf("no unique locations generated")
	}

	if _, err := tx.Exec(ctx, completeEventGenerationSQL, id, variant.Title, variant.Description); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (r *EventRepository) FailGeneration(ctx context.Context, eventID string, generationError string) error {
	id, err := parseUUID(eventID)
	if err != nil {
		return errorsstatus.ErrInvalidInput
	}

	tag, err := r.db.Exec(ctx, failEventGenerationSQL, id, generationError)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errorsstatus.ErrNotFound
	}

	return nil
}

func (r *EventRepository) SelectVariant(ctx context.Context, eventID string, variantID string) (core.Event, error) {
	eventUUID, err := parseUUID(eventID)
	if err != nil {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	variantUUID, err := parseUUID(variantID)
	if err != nil {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	tag, err := r.db.Exec(ctx, selectEventVariantSQL, eventUUID, variantUUID)
	if err != nil {
		return core.Event{}, err
	}
	if tag.RowsAffected() == 0 {
		return core.Event{}, errorsstatus.ErrNotFound
	}

	return r.GetByID(ctx, eventID)
}

func (r *EventRepository) ListMine(ctx context.Context, userID string) ([]core.Event, error) {
	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, errorsstatus.ErrInvalidInput
	}

	rows, err := r.queries.ListMyEvents(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	events := make([]core.Event, 0, len(rows))
	for _, row := range rows {
		event, err := mapListMyEventsRow(row)
		if err != nil {
			return nil, err
		}
		if err := r.loadVariants(ctx, r.queries, &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, nil
}

func (r *EventRepository) JoinByToken(ctx context.Context, params repo.JoinEventByTokenParams) (core.Event, error) {
	userID, err := parseUUID(params.UserID)
	if err != nil {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return core.Event{}, err
	}
	defer tx.Rollback(ctx)

	queries := r.queries.WithTx(tx)

	invite, err := queries.GetInviteByToken(ctx, params.Token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.Event{}, errorsstatus.ErrNotFound
		}
		return core.Event{}, err
	}

	profile, err := queries.GetUserProfileByID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.Event{}, errorsstatus.ErrNotFound
		}
		return core.Event{}, err
	}

	if err := queries.UpsertEventGuestByUser(ctx, dbsqlc.UpsertEventGuestByUserParams{
		EventID:  invite.EventID,
		InviteID: invite.ID,
		UserID:   userID,
		FullName: profile.FullName,
		Phone:    profile.Phone,
	}); err != nil {
		return core.Event{}, mapEventPgError(err)
	}

	if err := queries.IncrementInviteUsage(ctx, invite.ID); err != nil {
		return core.Event{}, err
	}

	row, err := queries.GetEventWithGuestAccess(ctx, dbsqlc.GetEventWithGuestAccessParams{
		UserID:  userID,
		EventID: invite.EventID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.Event{}, errorsstatus.ErrNotFound
		}
		return core.Event{}, err
	}

	event, err := mapGuestAccessEventRow(row)
	if err != nil {
		return core.Event{}, err
	}

	if err := r.loadVariants(ctx, queries, &event); err != nil {
		return core.Event{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return core.Event{}, err
	}

	return event, nil
}

func (r *EventRepository) GetByID(ctx context.Context, eventID string) (core.Event, error) {
	id, err := parseUUID(eventID)
	if err != nil {
		return core.Event{}, errorsstatus.ErrInvalidInput
	}

	row, err := r.queries.GetEventByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.Event{}, errorsstatus.ErrNotFound
		}
		return core.Event{}, err
	}

	event, err := mapGetEventByIDRow(row)
	if err != nil {
		return core.Event{}, err
	}

	if err := r.loadVariants(ctx, r.queries, &event); err != nil {
		return core.Event{}, err
	}

	return event, nil
}

func (r *EventRepository) GetAccessRole(ctx context.Context, userID string, eventID string) (string, error) {
	userUUID, err := parseUUID(userID)
	if err != nil {
		return "", errorsstatus.ErrInvalidInput
	}

	eventUUID, err := parseUUID(eventID)
	if err != nil {
		return "", errorsstatus.ErrInvalidInput
	}

	role, err := r.queries.GetEventAccessRole(ctx, dbsqlc.GetEventAccessRoleParams{
		EventID: eventUUID,
		UserID:  userUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errorsstatus.ErrForbidden
		}
		return "", err
	}

	return role, nil
}

func (r *EventRepository) loadVariants(ctx context.Context, queries *dbsqlc.Queries, event *core.Event) error {
	eventID, err := parseUUID(event.ID)
	if err != nil {
		return err
	}

	variantRows, err := queries.ListEventVariantsByEventID(ctx, eventID)
	if err != nil {
		return err
	}

	locationRows, err := queries.ListEventLocationsByEventID(ctx, eventID)
	if err != nil {
		return err
	}

	variants := make([]core.EventVariant, 0, len(variantRows))
	for _, row := range variantRows {
		variants = append(variants, mapSQLCVariant(row))
	}

	locationsByVariant := make(map[string][]core.EventLocation)
	for _, row := range locationRows {
		location, err := mapSQLCLocation(row)
		if err != nil {
			return err
		}
		locationsByVariant[location.VariantID] = append(locationsByVariant[location.VariantID], location)
	}

	for i := range variants {
		variants[i].Locations = locationsByVariant[variants[i].ID]
	}

	event.Variants = variants
	return nil
}

func (r *EventRepository) GetInviteToken(ctx context.Context, eventID string) (string, error) {
	id, err := parseUUID(eventID)
	if err != nil {
		return "", errorsstatus.ErrInvalidInput
	}

	token, err := r.queries.GetEventInviteByEventID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errorsstatus.ErrNotFound
		}
		return "", err
	}

	return token, nil
}

func (r *EventRepository) ListGuests(ctx context.Context, eventID string, approvalStatus *string) ([]core.EventGuest, error) {
	id, err := parseUUID(eventID)
	if err != nil {
		return nil, errorsstatus.ErrInvalidInput
	}

	rows, err := r.queries.ListEventGuests(ctx, dbsqlc.ListEventGuestsParams{
		EventID:        id,
		ApprovalStatus: approvalStatus,
	})
	if err != nil {
		return nil, err
	}

	guests := make([]core.EventGuest, 0, len(rows))
	for _, row := range rows {
		guests = append(guests, mapListEventGuestsRow(row))
	}

	return guests, nil
}

func (r *EventRepository) UpdateGuestAttendanceStatus(ctx context.Context, params repo.UpdateGuestAttendanceParams) (core.EventGuest, error) {
	guestID, err := parseUUID(params.GuestID)
	if err != nil {
		return core.EventGuest{}, errorsstatus.ErrInvalidInput
	}

	eventID, err := parseUUID(params.EventID)
	if err != nil {
		return core.EventGuest{}, errorsstatus.ErrInvalidInput
	}

	row, err := r.queries.UpdateGuestAttendanceStatus(ctx, dbsqlc.UpdateGuestAttendanceStatusParams{
		ID:               guestID,
		EventID:          eventID,
		AttendanceStatus: string(params.AttendanceStatus),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// ErrNoRows означает либо гость не найден, либо approval_status != 'approved'
			return core.EventGuest{}, errorsstatus.ErrForbidden
		}
		return core.EventGuest{}, err
	}

	return mapUpdateGuestAttendanceRow(row), nil
}

func (r *EventRepository) GetGuestStats(ctx context.Context, eventID string) (core.EventGuestStats, error) {
	id, err := parseUUID(eventID)
	if err != nil {
		return core.EventGuestStats{}, errorsstatus.ErrInvalidInput
	}

	row, err := r.queries.GetEventGuestStats(ctx, id)
	if err != nil {
		return core.EventGuestStats{}, err
	}

	return mapGuestStatsRow(row), nil
}

func mapEventPgError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return errorsstatus.ErrConflict
	}

	return err
}

func buildLocationDedupKey(title string, address *string) string {
	normalizedAddress := ""
	if address != nil {
		normalizedAddress = strings.ToLower(strings.TrimSpace(*address))
	}

	return strings.ToLower(strings.TrimSpace(title)) + "\n" + normalizedAddress
}

func (r *EventRepository) GetGuestAttendanceStatus(ctx context.Context, eventID string, userID string) (core.AttendanceStatus, error) {
	eUUID, err := parseUUID(eventID)
	if err != nil {
		return "", errorsstatus.ErrInvalidInput
	}
	uUUID, err := parseUUID(userID)
	if err != nil {
		return "", errorsstatus.ErrInvalidInput
	}

	var status string
	err = r.db.QueryRow(ctx, "SELECT attendance_status FROM event_guests WHERE event_id = $1 AND user_id = $2", eUUID, uUUID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errorsstatus.ErrNotFound
		}
		return "", err
	}

	return core.AttendanceStatus(status), nil
}
