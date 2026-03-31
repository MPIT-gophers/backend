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
