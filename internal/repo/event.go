package repo

import (
	"context"

	"eventAI/internal/entities/core"
)

type CreateEventParams struct {
	UserID             string
	City               string
	EventDate          string
	EventTime          string
	ExpectedGuestCount int
	Budget             string
}

type JoinEventByTokenParams struct {
	UserID string
	Token  string
}

type UpdateGuestAttendanceParams struct {
	GuestID          string
	EventID          string
	AttendanceStatus core.AttendanceStatus
}

type EventRepository interface {
	Create(ctx context.Context, params CreateEventParams) (core.Event, error)
	UpdateStatus(ctx context.Context, eventID string, status string) error
	ListMine(ctx context.Context, userID string) ([]core.Event, error)
	JoinByToken(ctx context.Context, params JoinEventByTokenParams) (core.Event, error)
	GetByID(ctx context.Context, eventID string) (core.Event, error)
	GetAccessRole(ctx context.Context, userID string, eventID string) (string, error)

	GetInviteToken(ctx context.Context, eventID string) (string, error)
	ListGuests(ctx context.Context, eventID string, approvalStatus *string) ([]core.EventGuest, error)
	UpdateGuestAttendanceStatus(ctx context.Context, params UpdateGuestAttendanceParams) (core.EventGuest, error)
	GetGuestStats(ctx context.Context, eventID string) (core.EventGuestStats, error)
	GetGuestAttendanceStatus(ctx context.Context, eventID string, userID string) (core.AttendanceStatus, error)
}
