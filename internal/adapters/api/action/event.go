package action

import (
	"encoding/json"
	"errors"
	"net/http"

	"eventAI/internal/adapters/api/middleware"
	"eventAI/internal/adapters/api/response"
	errorsstatus "eventAI/internal/errorsStatus"
	"eventAI/internal/service"

	"github.com/go-chi/chi/v5"
)

type EventHandler struct {
	service *service.EventService
}

func NewEventHandler(service *service.EventService) *EventHandler {
	return &EventHandler{service: service}
}

type CreateEventRequest struct {
	City               string `json:"city"`
	EventDate          string `json:"event_date"`
	EventTime          string `json:"event_time"`
	ExpectedGuestCount int    `json:"expected_guest_count"`
	Budget             string `json:"budget"`
}

// Create godoc
// @Summary Создать событие
// @Description Создаёт новое событие от имени текущего авторизованного пользователя.
// @Description Организатором события автоматически становится пользователь из Bearer JWT.
// @Description В запросе нужно передать базовые параметры события: город, дату, время, ожидаемое число гостей и бюджет.
// @Tags events
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateEventRequest true "Тело запроса с параметрами события"
// @Success 201 {object} response.SuccessEnvelope{data=core.Event} "Событие успешно создано"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный JSON или ошибка валидации полей"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 409 {object} response.ErrorEnvelope "Конфликт при создании события"
// @Router /events [post]
func (h *EventHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Failure(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req CreateEventRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid json body")
		return
	}

	event, err := h.service.Create(r.Context(), userID, service.CreateEventInput{
		City:               req.City,
		EventDate:          req.EventDate,
		EventTime:          req.EventTime,
		ExpectedGuestCount: req.ExpectedGuestCount,
		Budget:             req.Budget,
	})
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), eventErrorMessage(err))
		return
	}

	response.Success(w, http.StatusCreated, event)
}

// ListMine godoc
// @Summary Получить список моих событий
// @Description Возвращает список событий, к которым текущий пользователь имеет отношение.
// @Description В выборку попадают события, где пользователь является организатором, соорганизатором или гостем.
// @Description Для каждого события backend дополнительно возвращает access_role и связанные статусы гостя, если они применимы.
// @Tags events
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.SuccessEnvelope{data=[]core.Event} "Список событий текущего пользователя"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Router /events/my [get]
func (h *EventHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Failure(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	events, err := h.service.ListMine(r.Context(), userID)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), eventErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, events)
}

type JoinEventByTokenRequest struct {
	Token string `json:"token"`
}

// JoinByToken godoc
// @Summary Войти в событие по invite token
// @Description Добавляет текущего пользователя в событие по invite token.
// @Description Токен должен быть активным и принадлежать существующему событию.
// @Description После успешного входа backend возвращает карточку события с ролью доступа пользователя в этом событии.
// @Tags events
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body JoinEventByTokenRequest true "Тело запроса с invite token"
// @Success 200 {object} response.SuccessEnvelope{data=core.Event} "Пользователь успешно добавлен в событие"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный JSON или пустой token"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 404 {object} response.ErrorEnvelope "Invite token не найден или недействителен"
// @Failure 409 {object} response.ErrorEnvelope "Конфликт при присоединении к событию"
// @Router /events/join-by-token [post]
func (h *EventHandler) JoinByToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Failure(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req JoinEventByTokenRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid json body")
		return
	}

	event, err := h.service.JoinByToken(r.Context(), userID, req.Token)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), eventErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, event)
}

// GetByID godoc
// @Summary Получить событие по ID
// @Description Возвращает полную карточку события по его идентификатору.
// @Description Доступ к методу ограничен middleware авторизации и проверкой прав доступа к конкретному событию.
// @Description Если у пользователя нет права читать событие, backend вернёт 403.
// @Tags events
// @Produce json
// @Security BearerAuth
// @Param eventID path string true "Идентификатор события"
// @Success 200 {object} response.SuccessEnvelope{data=core.Event} "Карточка события"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный eventID"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 403 {object} response.ErrorEnvelope "Недостаточно прав для чтения события"
// @Failure 404 {object} response.ErrorEnvelope "Событие не найдено"
// @Router /events/{eventID} [get]
func (h *EventHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventID")
	event, err := h.service.GetByID(r.Context(), eventID)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), eventErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, event)
}

// GetInvite godoc
// @Summary Получить invite token события
// @Description Возвращает активный invite token для указанного события.
// @Description Обычно используется организатором или соорганизатором, чтобы передать ссылку или token приглашённому пользователю.
// @Tags events
// @Produce json
// @Security BearerAuth
// @Param eventID path string true "Идентификатор события"
// @Success 200 {object} response.SuccessEnvelope{data=object{token=string}} "Активный invite token события"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 403 {object} response.ErrorEnvelope "Недостаточно прав для просмотра invite token"
// @Failure 404 {object} response.ErrorEnvelope "Событие или invite token не найдены"
// @Router /events/{eventID}/invite [get]
func (h *EventHandler) GetInvite(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventID")

	token, err := h.service.GetInviteToken(r.Context(), eventID)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), eventErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, map[string]string{"token": token})
}

// ListGuests godoc
// @Summary Получить список гостей события
// @Description Возвращает список гостей, связанных с указанным событием.
// @Description Можно дополнительно отфильтровать список по approval_status через query-параметр.
// @Description Доступ к методу ограничен проверкой прав на чтение события.
// @Tags events
// @Produce json
// @Security BearerAuth
// @Param eventID path string true "Идентификатор события"
// @Param approval_status query string false "Фильтр по статусу подтверждения: pending, approved, rejected"
// @Success 200 {object} response.SuccessEnvelope{data=[]core.EventGuest} "Список гостей события"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 403 {object} response.ErrorEnvelope "Недостаточно прав для просмотра гостей"
// @Router /events/{eventID}/guests [get]
func (h *EventHandler) ListGuests(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventID")
	approvalStatus := r.URL.Query().Get("approval_status")

	guests, err := h.service.ListGuests(r.Context(), eventID, approvalStatus)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), eventErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, guests)
}

type UpdateGuestStatusRequest struct {
	AttendanceStatus *string `json:"attendance_status"`
}

// UpdateGuestStatus godoc
// @Summary Обновить статус посещения гостя
// @Description Обновляет attendance_status конкретного гостя в рамках события.
// @Description Метод предназначен для сценария, когда сам пользователь подтверждает или отклоняет своё участие.
// @Description Организатор и co_host не могут использовать этот endpoint для управления посещаемостью других пользователей.
// @Tags events
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param eventID path string true "Идентификатор события"
// @Param guestID path string true "Идентификатор гостя"
// @Param request body UpdateGuestStatusRequest true "Тело запроса с attendance_status"
// @Success 200 {object} response.SuccessEnvelope{data=core.EventGuest} "Статус посещения успешно обновлён"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный JSON, eventID, guestID или attendance_status"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 403 {object} response.ErrorEnvelope "Недостаточно прав для обновления статуса посещения"
// @Failure 404 {object} response.ErrorEnvelope "Гость не найден"
// @Router /events/{eventID}/guests/{guestID}/status [patch]
func (h *EventHandler) UpdateGuestStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Failure(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	eventID := chi.URLParam(r, "eventID")
	guestID := chi.URLParam(r, "guestID")

	var req UpdateGuestStatusRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid json body")
		return
	}

	guest, err := h.service.UpdateGuestStatus(r.Context(), userID, eventID, guestID, service.UpdateGuestStatusInput{
		AttendanceStatus: req.AttendanceStatus,
	})
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), eventErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, guest)
}

// GetGuestStats godoc
// @Summary Получить статистику по гостям события
// @Description Возвращает агрегированную статистику по гостям указанного события.
// @Description В ответ включаются счётчики по approval_status и attendance_status.
// @Description Метод полезен для дашбордов, карточек события и административных экранов.
// @Tags events
// @Produce json
// @Security BearerAuth
// @Param eventID path string true "Идентификатор события"
// @Success 200 {object} response.SuccessEnvelope{data=core.EventGuestStats} "Статистика по гостям события"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 403 {object} response.ErrorEnvelope "Недостаточно прав для просмотра статистики"
// @Router /events/{eventID}/stats [get]
func (h *EventHandler) GetGuestStats(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventID")

	stats, err := h.service.GetGuestStats(r.Context(), eventID)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), eventErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, stats)
}

func eventErrorMessage(err error) string {
	switch {
	case errors.Is(err, errorsstatus.ErrInvalidInput):
		return "invalid input"
	case errors.Is(err, errorsstatus.ErrUnauthorized):
		return "unauthorized"
	case errors.Is(err, errorsstatus.ErrForbidden):
		return "forbidden"
	case errors.Is(err, errorsstatus.ErrConflict):
		return "conflict"
	case errors.Is(err, errorsstatus.ErrNotFound):
		return "not found"
	default:
		return "internal server error"
	}
}
