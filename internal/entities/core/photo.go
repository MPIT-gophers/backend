package core

import (
	"time"

	"github.com/google/uuid"
)

type EventPhoto struct {
	ID        uuid.UUID  `json:"id"`
	EventID   uuid.UUID  `json:"event_id"`
	GuestID   *uuid.UUID `json:"guest_id,omitempty"`
	FilePath  string     `json:"file_path"`
	SizeBytes int64      `json:"size_bytes"`
	MimeType  string     `json:"mime_type"`
	CreatedAt time.Time  `json:"created_at"`
}
