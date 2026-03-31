package database

import (
	"context"
	"errors"
	"time"

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

type AuthRepository struct {
	db      *pgxpool.Pool
	queries *dbsqlc.Queries
}

func NewAuthRepository(db *pgxpool.Pool) repo.AuthRepository {
	return &AuthRepository{
		db:      db,
		queries: dbsqlc.New(db),
	}
}

func (r *AuthRepository) AuthenticateWithOAuth(ctx context.Context, params repo.AuthenticateWithOAuthParams) (core.User, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return core.User{}, err
	}
	defer tx.Rollback(ctx)

	queries := r.queries.WithTx(tx)

	user, err := queries.GetUserByOAuth(ctx, dbsqlc.GetUserByOAuthParams{
		Provider:       params.Provider,
		ProviderUserID: params.ProviderUserID,
	})
	if err == nil {
		user, err = enrichUserFromProvider(ctx, queries, user, params.FullName, params.Phone)
		if err != nil {
			return core.User{}, mapAuthPgError(err)
		}
		if err := tx.Commit(ctx); err != nil {
			return core.User{}, err
		}
		return mapSQLCUser(user), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return core.User{}, err
	}

	if params.Phone != nil {
		user, err = queries.GetUserByPhone(ctx, params.Phone)
		if err == nil {
			if err := queries.InsertUserOAuthAccount(ctx, dbsqlc.InsertUserOAuthAccountParams{
				UserID:         user.ID,
				Provider:       params.Provider,
				ProviderUserID: params.ProviderUserID,
			}); err != nil {
				return core.User{}, mapAuthPgError(err)
			}

			user, err = enrichUserFromProvider(ctx, queries, user, params.FullName, params.Phone)
			if err != nil {
				return core.User{}, mapAuthPgError(err)
			}
			if err := tx.Commit(ctx); err != nil {
				return core.User{}, err
			}
			return mapSQLCUser(user), nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return core.User{}, err
		}
	}

	user, err = queries.CreateUser(ctx, dbsqlc.CreateUserParams{
		FullName: params.FullName,
		Phone:    params.Phone,
	})
	if err != nil {
		return core.User{}, mapAuthPgError(err)
	}

	if err := queries.InsertUserOAuthAccount(ctx, dbsqlc.InsertUserOAuthAccountParams{
		UserID:         user.ID,
		Provider:       params.Provider,
		ProviderUserID: params.ProviderUserID,
	}); err != nil {
		return core.User{}, mapAuthPgError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return core.User{}, err
	}

	return mapSQLCUser(user), nil
}

func (r *AuthRepository) CreateMAXAuthSession(ctx context.Context, params repo.CreateMAXAuthSessionParams) (core.MAXAuthSession, error) {
	const query = `
		INSERT INTO auth_sessions (
			provider,
			status,
			expires_at
		) VALUES (
			'max',
			'pending',
			$1
		)
		RETURNING
			id,
			status,
			expires_at,
			completed_at,
			exchanged_at,
			provider_user_id,
			user_id;
	`

	return scanMAXAuthSession(r.db.QueryRow(ctx, query, params.ExpiresAt))
}

func (r *AuthRepository) GetMAXAuthSession(ctx context.Context, sessionID string) (core.MAXAuthSession, error) {
	id, err := parseUUID(sessionID)
	if err != nil {
		return core.MAXAuthSession{}, errorsstatus.ErrInvalidInput
	}

	const query = `
		SELECT
			id,
			status,
			expires_at,
			completed_at,
			exchanged_at,
			provider_user_id,
			user_id
		FROM auth_sessions
		WHERE id = $1
		  AND provider = 'max';
	`

	session, err := scanMAXAuthSession(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.MAXAuthSession{}, errorsstatus.ErrNotFound
		}
		return core.MAXAuthSession{}, err
	}

	return session, nil
}

func (r *AuthRepository) CompleteMAXAuthSession(ctx context.Context, params repo.CompleteMAXAuthSessionParams) (core.MAXAuthSession, error) {
	sessionID, err := parseUUID(params.SessionID)
	if err != nil {
		return core.MAXAuthSession{}, errorsstatus.ErrInvalidInput
	}

	userID, err := parseUUID(params.UserID)
	if err != nil {
		return core.MAXAuthSession{}, errorsstatus.ErrInvalidInput
	}

	const query = `
		UPDATE auth_sessions
		SET
			status = 'completed',
			provider_user_id = $2,
			user_id = $3,
			completed_at = $4,
			updated_at = $4
		WHERE id = $1
		  AND provider = 'max'
		  AND status = 'pending'
		  AND expires_at > $5
		RETURNING
			id,
			status,
			expires_at,
			completed_at,
			exchanged_at,
			provider_user_id,
			user_id;
	`

	session, err := scanMAXAuthSession(r.db.QueryRow(
		ctx,
		query,
		sessionID,
		params.ProviderUserID,
		userID,
		params.CompletedAt,
		params.CompletedAt,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.MAXAuthSession{}, errorsstatus.ErrConflict
		}
		return core.MAXAuthSession{}, err
	}

	return session, nil
}

func (r *AuthRepository) ExchangeMAXAuthSession(ctx context.Context, params repo.ExchangeMAXAuthSessionParams) (core.MAXAuthSession, error) {
	sessionID, err := parseUUID(params.SessionID)
	if err != nil {
		return core.MAXAuthSession{}, errorsstatus.ErrInvalidInput
	}

	const query = `
		UPDATE auth_sessions
		SET
			status = 'exchanged',
			exchanged_at = $2,
			updated_at = $2
		WHERE id = $1
		  AND provider = 'max'
		  AND status = 'completed'
		RETURNING
			id,
			status,
			expires_at,
			completed_at,
			exchanged_at,
			provider_user_id,
			user_id;
	`

	session, err := scanMAXAuthSession(r.db.QueryRow(ctx, query, sessionID, params.ExchangedAt))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.MAXAuthSession{}, errorsstatus.ErrConflict
		}
		return core.MAXAuthSession{}, err
	}

	return session, nil
}

func (r *AuthRepository) GetUserByID(ctx context.Context, userID string) (core.User, error) {
	id, err := parseUUID(userID)
	if err != nil {
		return core.User{}, errorsstatus.ErrInvalidInput
	}

	const query = `
		SELECT
			id,
			full_name,
			phone,
			created_at,
			updated_at
		FROM users
		WHERE id = $1;
	`

	var (
		dbID      pgtype.UUID
		fullName  string
		phone     *string
		createdAt time.Time
		updatedAt time.Time
	)

	if err := r.db.QueryRow(ctx, query, id).Scan(&dbID, &fullName, &phone, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.User{}, errorsstatus.ErrNotFound
		}
		return core.User{}, err
	}

	return core.User{
		ID:        dbID.String(),
		FullName:  fullName,
		Phone:     phone,
		CreatedAt: createdAt.UTC(),
		UpdatedAt: updatedAt.UTC(),
	}, nil
}

func (r *AuthRepository) UpdateUserProfile(ctx context.Context, params repo.UpdateUserProfileParams) (core.User, error) {
	userID, err := parseUUID(params.UserID)
	if err != nil {
		return core.User{}, errorsstatus.ErrInvalidInput
	}

	fullName := ""
	if params.FullName != nil {
		fullName = *params.FullName
	}

	user, err := r.queries.UpdateUserProfile(ctx, dbsqlc.UpdateUserProfileParams{
		FullNameSet: params.FullNameSet,
		FullName:    fullName,
		PhoneSet:    params.PhoneSet,
		Phone:       params.Phone,
		ID:          userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.User{}, errorsstatus.ErrNotFound
		}
		return core.User{}, mapAuthPgError(err)
	}

	return mapSQLCUser(user), nil
}

func enrichUserFromProvider(
	ctx context.Context,
	queries *dbsqlc.Queries,
	user dbsqlc.User,
	fullName string,
	phone *string,
) (dbsqlc.User, error) {
	shouldUpdateName := user.FullName == service.UnsetFullName && fullName != service.UnsetFullName
	shouldUpdatePhone := user.Phone == nil && phone != nil
	if !shouldUpdateName && !shouldUpdatePhone {
		return user, nil
	}

	return queries.EnrichUserFromProvider(ctx, dbsqlc.EnrichUserFromProviderParams{
		FullNameSet: shouldUpdateName,
		FullName:    fullName,
		PhoneSet:    shouldUpdatePhone,
		Phone:       phone,
		ID:          user.ID,
	})
}

func mapAuthPgError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return errorsstatus.ErrConflict
	}

	return err
}

func scanMAXAuthSession(row pgx.Row) (core.MAXAuthSession, error) {
	var (
		id             pgtype.UUID
		status         string
		expiresAt      time.Time
		completedAt    *time.Time
		exchangedAt    *time.Time
		providerUserID *string
		userID         pgtype.UUID
	)

	if err := row.Scan(
		&id,
		&status,
		&expiresAt,
		&completedAt,
		&exchangedAt,
		&providerUserID,
		&userID,
	); err != nil {
		return core.MAXAuthSession{}, err
	}

	session := core.MAXAuthSession{
		SessionID: id.String(),
		Status:    status,
		ExpiresAt: expiresAt.UTC(),
	}
	if completedAt != nil {
		completedAtUTC := completedAt.UTC()
		session.CompletedAt = &completedAtUTC
	}
	if exchangedAt != nil {
		exchangedAtUTC := exchangedAt.UTC()
		session.ExchangedAt = &exchangedAtUTC
	}
	if providerUserID != nil {
		session.ProviderUserID = *providerUserID
	}
	if userID.Valid {
		session.UserID = userID.String()
	}

	return session, nil
}
