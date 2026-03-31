package repo

import (
	"context"

	"eventAI/internal/entities/core"
)

type CreateRegistrationParams struct {
	FullName string
	Email    string
}

type RegistrationRepository interface {
	Create(ctx context.Context, input CreateRegistrationParams) (core.Registration, error)
	List(ctx context.Context) ([]core.Registration, error)
}
