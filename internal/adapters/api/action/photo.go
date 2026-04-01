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

// UploadPhotos godoc
// @Summary Загрузить фотографии события
// @Description Принимает multipart/form-data с набором файлов и загружает фотографии в контекст указанного события.
// @Description За один запрос можно загрузить не более 10 файлов.
// @Description Метод предназначен для гостей и организаторов события; размер входного multipart-пакета ограничен backend-логикой.
// @Tags photos
// @Accept mpfd
// @Produce json
// @Param eventID path string true "Идентификатор события"
// @Param photos formData file true "Один или несколько файлов фотографий"
// @Success 200 {object} map[string]interface{} "Фотографии успешно загружены"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный eventID, отсутствие файлов или превышение лимита количества"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 413 {object} response.ErrorEnvelope "Слишком большой multipart-запрос"
// @Failure 207 {object} response.ErrorEnvelope "Часть файлов загрузилась, часть завершилась ошибкой"
// @Router /api/v1/events/{eventID}/photos/upload [post]
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

// GetPhotos godoc
// @Summary Получить фотографии события
// @Description Возвращает список URL фотографий, привязанных к событию.
// @Description Доступ к фотографиям есть только у пользователей, которым разрешено читать событие и связанные с ним материалы.
// @Description Если у пользователя недостаточно прав, backend вернёт 403.
// @Tags photos
// @Produce json
// @Param eventID path string true "Идентификатор события"
// @Success 200 {object} map[string]interface{} "Массив URL фотографий события"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный eventID"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 403 {object} response.ErrorEnvelope "Недостаточно прав для просмотра фотографий"
// @Failure 500 {object} response.ErrorEnvelope "Внутренняя ошибка при получении списка фотографий"
// @Router /api/v1/events/{eventID}/photos [get]
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
