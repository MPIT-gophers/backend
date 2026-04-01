package action

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"eventAI/internal/adapters/api/middleware"
	"eventAI/internal/adapters/api/response"
	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/service"
)

type PhotoHandler struct {
	photoService service.PhotoService
}

func NewPhotoHandler(s service.PhotoService) *PhotoHandler {
	return &PhotoHandler{
		photoService: s,
	}
}

func (h *PhotoHandler) UploadPhotos(w http.ResponseWriter, r *http.Request) {
	eventIDStr := chi.URLParam(r, "eventID")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid event ID format")
		return
	}

	userIDStr, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Failure(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid user ID format")
		return
	}

	// 100 MB max payload for the form
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		response.Failure(w, http.StatusRequestEntityTooLarge, "request too large or invalid multipart")
		return
	}
	defer r.MultipartForm.RemoveAll()

	files := r.MultipartForm.File["photos"]
	if len(files) == 0 {
		response.Failure(w, http.StatusBadRequest, "no photos provided in 'photos' field")
		return
	}

	if len(files) > 10 {
		response.Failure(w, http.StatusBadRequest, "maximum 10 photos per batch allowed")
		return
	}

	var successCount int
	var errs []string

	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			errs = append(errs, "failed to open file: "+fileHeader.Filename)
			continue
		}

		fileBytes, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			errs = append(errs, "failed to read file: "+fileHeader.Filename)
			continue
		}

		mimeType := http.DetectContentType(fileBytes)

		err = h.photoService.UploadPhoto(r.Context(), eventID, userID, fileBytes, mimeType)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", fileHeader.Filename, err.Error()))
		} else {
			successCount++
		}
	}

	if len(errs) > 0 {
		response.Failure(w, http.StatusMultiStatus, fmt.Sprintf("uploaded %d photos with errors: %v", successCount, errs))
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Photos uploaded successfully",
	})
}

// GetPhotos returns a list of photo URLs for the event.
// Only accessible to confirmed guests or organizers.
func (h *PhotoHandler) GetPhotos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eventIDStr := chi.URLParam(r, "eventID")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		response.Failure(w, http.StatusBadRequest, errorsstatus.ErrInvalidInput.Error())
		return
	}

	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		response.Failure(w, http.StatusUnauthorized, errorsstatus.ErrUnauthorized.Error())
		return
	}

	userIDUUID, err := uuid.Parse(userID)
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, errorsstatus.ErrUnauthorized.Error())
		return
	}

	urls, err := h.photoService.GetEventPhotos(ctx, eventID, userIDUUID)
	if err != nil {
		if errors.Is(err, errorsstatus.ErrForbidden) {
			response.Failure(w, http.StatusForbidden, err.Error())
			return
		}
		response.Failure(w, http.StatusInternalServerError, err.Error())
		return
	}

	// If urls is nil, make it an empty array for consistent JSON response
	if urls == nil {
		urls = make([]string, 0)
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"photos": urls,
	})
}
