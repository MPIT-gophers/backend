package database

import (
	"context"
	"errors"

	"eventAI/internal/entities/core"
	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/infrastructure/database/sqlc"
	"eventAI/internal/repo"

	"github.com/jackc/pgx/v5/pgconn"
)

type RegistrationRepository struct {
	queries *sqlc.Queries
}

func NewRegistrationRepository(db sqlc.DBTX) repo.RegistrationRepository {
	return &RegistrationRepository{
		queries: sqlc.New(db),
	}
}

func (r *RegistrationRepository) Create(ctx context.Context, input repo.CreateRegistrationParams) (core.Registration, error) {
	registration, err := r.queries.CreateRegistration(ctx, sqlc.CreateRegistrationParams{
		FullName: input.FullName,
		Email:    input.Email,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return core.Registration{}, errorsstatus.ErrConflict
		}

		return core.Registration{}, err
	}

	return core.Registration{
		ID:        registration.ID,
		FullName:  registration.FullName,
		Email:     registration.Email,
		CreatedAt: registration.CreatedAt,
	}, nil
}

func (r *RegistrationRepository) List(ctx context.Context) ([]core.Registration, error) {
	items, err := r.queries.ListRegistrations(ctx)
	if err != nil {
		return nil, err
	}

	registrations := make([]core.Registration, 0, len(items))
	for _, item := range items {
		registrations = append(registrations, core.Registration{
			ID:        item.ID,
			FullName:  item.FullName,
			Email:     item.Email,
			CreatedAt: item.CreatedAt,
		})
	}

	return registrations, nil
}
