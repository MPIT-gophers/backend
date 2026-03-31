package database

import (
	"context"
	"errors"

	"eventAI/internal/entities/core"
	errorsstatus "eventAI/internal/errorsStatus"
	dbsqlc "eventAI/internal/infrastructure/database/sqlc"
	"eventAI/internal/repo"
	"eventAI/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventRepository struct {
	db      *pgxpool.Pool
	queries *dbsqlc.Queries
}

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

	row, err := queries.CreateEvent(ctx, dbsqlc.CreateEventParams{
		City:               params.City,
		EventDate:          eventDate,
		EventTime:          eventTime,
		ExpectedGuestCount: int32(params.ExpectedGuestCount),
		Budget:             budget,
	})
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

func mapEventPgError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return errorsstatus.ErrConflict
	}

	return err
}
