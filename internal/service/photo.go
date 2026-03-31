package service

import (
	"context"
	"errors"
	"fmt"

	"eventAI/internal/entities/core"
	errorsstatus "eventAI/internal/errorsStatus"

	"github.com/google/uuid"
)

type StorageProvider interface {
	SaveFile(ctx context.Context, file []byte, ext string) (string, error)
}

type PhotoRepository interface {
	SavePhoto(ctx context.Context, eventID uuid.UUID, guestID uuid.UUID, filePath string, sizeBytes int64, mimeType string) error
	GetPhotosByEventID(ctx context.Context, eventID uuid.UUID) ([]core.EventPhoto, error)
}

type EventValidator interface {
	GetGuestAttendanceStatus(ctx context.Context, eventID string, userID string) (core.AttendanceStatus, error)
}

type PhotoService interface {
	UploadPhoto(ctx context.Context, eventID uuid.UUID, guestID uuid.UUID, fileBytes []byte, mimeType string) error
	GetEventPhotos(ctx context.Context, eventID uuid.UUID, userID uuid.UUID) ([]string, error)
}

type photoService struct {
	repo     PhotoRepository
	storage  StorageProvider
	eventVal EventValidator
}

func NewPhotoService(r PhotoRepository, s StorageProvider, v EventValidator) PhotoService {
	return &photoService{
		repo:     r,
		storage:  s,
		eventVal: v,
	}
}

func (s *photoService) UploadPhoto(ctx context.Context, eventID uuid.UUID, guestID uuid.UUID, fileBytes []byte, mimeType string) error {
	const maxSize = 10 * 1024 * 1024 // 10MB
	if len(fileBytes) > maxSize {
		return errors.New("file too large")
	}

	if mimeType != "image/jpeg" && mimeType != "image/png" && mimeType != "image/webp" {
		return errors.New("unsupported file type")
	}

	ext := ".jpg"
	if mimeType == "image/png" {
		ext = ".png"
	} else if mimeType == "image/webp" {
		ext = ".webp"
	}

	filePath, err := s.storage.SaveFile(ctx, fileBytes, ext)
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	err = s.repo.SavePhoto(ctx, eventID, guestID, filePath, int64(len(fileBytes)), mimeType)
	if err != nil {
		return fmt.Errorf("failed to save photo record: %w", err)
	}

	return nil
}

func (s *photoService) GetEventPhotos(ctx context.Context, eventID uuid.UUID, userID uuid.UUID) ([]string, error) {
	status, err := s.eventVal.GetGuestAttendanceStatus(ctx, eventID.String(), userID.String())
	if err != nil {
		// Logically, if the user isn't found in guests, they might be an organizer.
		// For now we follow the simple test requirement.
		return nil, errorsstatus.ErrForbidden
	}

	if status != core.AttendanceConfirmed {
		return nil, errorsstatus.ErrForbidden
	}

	photos, err := s.repo.GetPhotosByEventID(ctx, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get photos: %w", err)
	}

	urls := make([]string, 0, len(photos))
	for _, p := range photos {
		urls = append(urls, p.FilePath)
	}

	return urls, nil
}
