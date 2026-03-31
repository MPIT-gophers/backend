package usecase

import (
	"context"
	"strings"

	"eventAI/internal/entities/core"
	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/infrastructure/validation"
	"eventAI/internal/repo"
)

type CreateRegistrationInput struct {
	FullName string
	Email    string
}

type RegistrationUseCase struct {
	repo repo.RegistrationRepository
}

func NewRegistrationUseCase(repo repo.RegistrationRepository) *RegistrationUseCase {
	return &RegistrationUseCase{repo: repo}
}

func (u *RegistrationUseCase) Create(ctx context.Context, input CreateRegistrationInput) (core.Registration, error) {
	fullName := strings.TrimSpace(input.FullName)
	email := strings.ToLower(strings.TrimSpace(input.Email))

	if fullName == "" || !validation.IsEmail(email) {
		return core.Registration{}, errorsstatus.ErrInvalidInput
	}

	return u.repo.Create(ctx, repo.CreateRegistrationParams{
		FullName: fullName,
		Email:    email,
	})
}

func (u *RegistrationUseCase) List(ctx context.Context) ([]core.Registration, error) {
	return u.repo.List(ctx)
}
