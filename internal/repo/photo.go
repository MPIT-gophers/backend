package repo

import (
	"context"

	"eventAI/internal/entities/core"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type photoRepository struct {
	db *pgxpool.Pool
}

func NewPhotoRepository(db *pgxpool.Pool) *photoRepository {
	return &photoRepository{db: db}
}

func (r *photoRepository) SavePhoto(ctx context.Context, eventID uuid.UUID, guestID uuid.UUID, filePath string, sizeBytes int64, mimeType string) error {
	query := `
		INSERT INTO event_photos (event_id, guest_id, file_path, size_bytes, mime_type)
		VALUES ($1, $2, $3, $4, $5)
	`
	// Handle nil guestID safely (for example, if organizer uploads)
	var gID interface{}
	if guestID != uuid.Nil {
		gID = guestID
	} else {
		gID = nil
	}

	_, err := r.db.Exec(ctx, query, eventID, gID, filePath, sizeBytes, mimeType)
	return err
}

func (r *photoRepository) GetPhotosByEventID(ctx context.Context, eventID uuid.UUID) ([]core.EventPhoto, error) {
	// Not implemented yet
	return nil, nil
}
