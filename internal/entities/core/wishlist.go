package core

import (
	"time"

	"github.com/google/uuid"
)

type WishlistItem struct {
	ID               uuid.UUID  `json:"id"`
	EventID          uuid.UUID  `json:"event_id"`
	Name             string     `json:"name"`
	EstimatedPrice   *float64   `json:"estimated_price,omitempty"`
	CurrentFund      float64    `json:"current_fund"`
	IsBooked         bool       `json:"is_booked"`
	BookedByGuestID  *uuid.UUID `json:"booked_by_guest_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type AntiWishlistItem struct {
	ID        uuid.UUID `json:"id"`
	EventID   uuid.UUID `json:"event_id"`
	StopWord  string    `json:"stop_word"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
