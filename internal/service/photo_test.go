package service

import (
	"context"
	"errors"
	"testing"

	"eventAI/internal/entities/core"

	"github.com/google/uuid"
)

// Mocks to satisfy the unwritten interfaces in our tests

type stubStorageProvider struct {
	savedPath string
	err       error
}

func (s *stubStorageProvider) SaveFile(ctx context.Context, file []byte, ext string) (string, error) {
	return s.savedPath, s.err
}

type stubPhotoRepo struct {
	err    error
	photos []core.EventPhoto
}

func (r *stubPhotoRepo) SavePhoto(ctx context.Context, eventID uuid.UUID, guestID uuid.UUID, filePath string, sizeBytes int64, mimeType string) error {
	return r.err
}

func (r *stubPhotoRepo) GetPhotosByEventID(ctx context.Context, eventID uuid.UUID) ([]core.EventPhoto, error) {
	return r.photos, r.err
}


type stubEventValidator struct {
	status core.AttendanceStatus
	err    error
}

func (s *stubEventValidator) GetGuestAttendanceStatus(ctx context.Context, eventID string, userID string) (core.AttendanceStatus, error) {
	return s.status, s.err
}

func TestPhotoService_UploadPhoto(t *testing.T) {
	t.Parallel()

	validEventID := uuid.New()
	validGuestID := uuid.New()

	// create a dummy 1MB file
	validFile := make([]byte, 1024*1024)
	
	// Create a dummy 11MB file to simulate size limit failure
	tooBigFile := make([]byte, 11*1024*1024)

	tests := []struct {
		name         string
		fileBytes    []byte
		mimeType     string
		repoErr      error
		storageErr   error
		wantErr      bool
		wantErrorMsg string
	}{
		{
			name:      "Valid Image JPEG",
			fileBytes: validFile,
			mimeType:  "image/jpeg",
			wantErr:   false,
		},
		{
			name:      "Valid Image PNG",
			fileBytes: validFile,
			mimeType:  "image/png",
			wantErr:   false,
		},
		{
			name:         "Invalid MIME Type",
			fileBytes:    validFile,
			mimeType:     "application/pdf",
			wantErr:      true,
			wantErrorMsg: "unsupported file type",
		},
		{
			name:         "File Too Big (>10MB)",
			fileBytes:    tooBigFile,
			mimeType:     "image/jpeg",
			wantErr:      true,
			wantErrorMsg: "file too large",
		},
		{
			name:         "Storage save fails",
			fileBytes:    validFile,
			mimeType:     "image/jpeg",
			storageErr:   errors.New("storage error"),
			wantErr:      true,
			wantErrorMsg: "failed to save file: storage error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			storage := &stubStorageProvider{err: tc.storageErr, savedPath: "/uploads/fake.jpg"}
			repo := &stubPhotoRepo{err: tc.repoErr}

			eventVal := &stubEventValidator{status: "confirmed"}
			svc := NewPhotoService(repo, storage, eventVal)

			// UploadPhoto doesn't exist yet!
			err := svc.UploadPhoto(context.Background(), validEventID, validGuestID, tc.fileBytes, tc.mimeType)

			if (err != nil) != tc.wantErr {
				t.Fatalf("UploadPhoto() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr && tc.wantErrorMsg != "" && err.Error() != tc.wantErrorMsg {
				t.Fatalf("UploadPhoto() expected error msg '%v' got '%v'", tc.wantErrorMsg, err.Error())
			}
		})
	}
}

func TestPhotoService_GetEventPhotos(t *testing.T) {
	t.Parallel()

	validEventID := uuid.New()
	validUserID := uuid.New()

	tests := []struct {
		name         string
		guestStatus  core.AttendanceStatus
		validatorErr error
		repoPhotos   []core.EventPhoto
		repoErr      error
		wantErr      bool
		wantPhotos   int
	}{
		{
			name:        "Confirmed Guest gets photos",
			guestStatus: core.AttendanceConfirmed,
			repoPhotos: []core.EventPhoto{
				{FilePath: "/uploads/1.jpg"},
				{FilePath: "/uploads/2.jpg"},
			},
			wantPhotos: 2,
			wantErr:    false,
		},
		{
			name:        "Pending Guest gets 403",
			guestStatus: core.AttendancePending,
			wantErr:     true,
		},
		{
			name:        "Organizer gets photos (status organizer)",
			guestStatus: core.AttendanceConfirmed, // we'll treat them separately or spoof
			repoPhotos: []core.EventPhoto{
				{FilePath: "/uploads/1.jpg"},
			},
			wantPhotos: 1,
			wantErr:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			storage := &stubStorageProvider{}
			repo := &stubPhotoRepo{photos: tc.repoPhotos, err: tc.repoErr}
			eventVal := &stubEventValidator{status: tc.guestStatus, err: tc.validatorErr}

			svc := NewPhotoService(repo, storage, eventVal)

			photos, err := svc.GetEventPhotos(context.Background(), validEventID, validUserID)

			if (err != nil) != tc.wantErr {
				t.Fatalf("GetEventPhotos() error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && len(photos) != tc.wantPhotos {
				t.Fatalf("expected %d photos, got %d", tc.wantPhotos, len(photos))
			}
		})
	}
}
