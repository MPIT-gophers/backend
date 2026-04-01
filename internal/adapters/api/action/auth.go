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

type AuthHandler struct {
	service *service.AuthService
}

func NewAuthHandler(service *service.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

type LoginWithMAXRequest struct {
	InitData string `json:"init_data"`
}

type CompleteMAXAuthRequest struct {
	SessionID string `json:"session_id"`
	InitData  string `json:"init_data"`
}

type ExchangeMAXAuthRequest struct {
	SessionID string `json:"session_id"`
}

// StartMAXAuth godoc
// @Summary Запустить мобильный auth flow через MAX mini app
// @Description Создаёт временную auth session для мобильного сценария входа через MAX.
// @Description В ответе возвращается session_id и deep link на MAX mini app.
// @Description Пользователь должен открыть ссылку, подтвердить вход внутри MAX, после чего фронтенд или backend может опрашивать статус сессии.
// @Description Этот метод полезен, когда вход начинается не из уже открытого mini app, а из внешнего клиента.
// @Tags auth
// @Produce json
// @Success 201 {object} response.SuccessEnvelope{data=core.MAXAuthStart} "Создана auth session и ссылка для входа"
// @Failure 503 {object} response.ErrorEnvelope "MAX auth недоступен: не настроен bot username или недоступен внешний MAX API"
// @Router /auth/max/start [post]
func (h *AuthHandler) StartMAXAuth(w http.ResponseWriter, r *http.Request) {
	start, err := h.service.StartMAXAuth(r.Context())
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), authErrorMessage(err))
		return
	}

	response.Success(w, http.StatusCreated, start)
}

// LoginWithMAX godoc
// @Summary Войти по MAX init_data и сразу получить JWT
// @Description Упрощённый сценарий авторизации для случая, когда frontend уже работает внутри MAX mini app.
// @Description Метод принимает init_data из window.WebApp.initData, валидирует подпись, срок жизни данных и идентичность пользователя.
// @Description Если пользователь ещё не существует в системе, он будет создан автоматически через OAuth-привязку.
// @Description В ответе возвращается локальный JWT и профиль пользователя.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginWithMAXRequest true "Тело запроса с init_data из MAX mini app"
// @Success 200 {object} response.SuccessEnvelope{data=core.AuthSession} "JWT успешно выдан"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный JSON или пустой init_data"
// @Failure 401 {object} response.ErrorEnvelope "init_data невалиден, просрочен или не прошёл проверку подписи"
// @Failure 409 {object} response.ErrorEnvelope "Конфликт бизнес-логики авторизации"
// @Router /auth/max/login [post]
func (h *AuthHandler) LoginWithMAX(w http.ResponseWriter, r *http.Request) {
	var req LoginWithMAXRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid json body")
		return
	}

	session, err := h.service.LoginWithMAX(r.Context(), req.InitData)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), authErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, session)
}

// CompleteMAXAuth godoc
// @Summary Завершить мобильный auth flow через MAX init_data
// @Description Второй шаг мобильного сценария входа.
// @Description Метод принимает session_id, созданный через /auth/max/start, и init_data, полученный после открытия MAX mini app.
// @Description Backend проверяет подпись MAX, срок действия init_data и соответствие start_param созданной auth session.
// @Description После успешной проверки session переходит в состояние completed, но JWT ещё не выдаётся.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body CompleteMAXAuthRequest true "Тело запроса с session_id и init_data из MAX"
// @Success 200 {object} response.SuccessEnvelope{data=core.MAXAuthSession} "Auth session успешно завершена"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный JSON или обязательные поля отсутствуют"
// @Failure 401 {object} response.ErrorEnvelope "init_data невалиден, просрочен или start_param не соответствует session_id"
// @Failure 404 {object} response.ErrorEnvelope "Auth session не найдена"
// @Failure 409 {object} response.ErrorEnvelope "Сессия уже завершена, уже обменяна на JWT или истекла"
// @Router /auth/max/complete [post]
func (h *AuthHandler) CompleteMAXAuth(w http.ResponseWriter, r *http.Request) {
	var req CompleteMAXAuthRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid json body")
		return
	}

	session, err := h.service.CompleteMAXAuth(r.Context(), req.SessionID, req.InitData)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), authErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, session)
}

// GetMAXAuthSession godoc
// @Summary Получить статус MAX auth session
// @Description Возвращает текущее состояние auth session, созданной через /auth/max/start.
// @Description Используется для polling-сценария, когда один клиент ждёт, пока пользователь завершит вход в MAX на другом устройстве или в другом окне.
// @Description Возможные статусы описаны в core.MAXAuthSession.
// @Tags auth
// @Produce json
// @Param sessionID path string true "ID auth session"
// @Success 200 {object} response.SuccessEnvelope{data=core.MAXAuthSession} "Текущий статус auth session"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный session_id"
// @Failure 404 {object} response.ErrorEnvelope "Auth session не найдена"
// @Router /auth/max/session/{sessionID} [get]
func (h *AuthHandler) GetMAXAuthSession(w http.ResponseWriter, r *http.Request) {
	session, err := h.service.GetMAXAuthSession(r.Context(), chi.URLParam(r, "sessionID"))
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), authErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, session)
}

// ExchangeMAXAuth godoc
// @Summary Обменять completed auth session на локальный JWT
// @Description Финальный шаг мобильного auth flow.
// @Description Метод принимает session_id, проверяет, что auth session уже находится в состоянии completed, и только после этого выдаёт JWT backend.
// @Description После успешного обмена повторное использование той же session запрещено.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body ExchangeMAXAuthRequest true "Тело запроса с session_id для обмена на JWT"
// @Success 200 {object} response.SuccessEnvelope{data=core.AuthSession} "JWT успешно выдан"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный JSON или пустой session_id"
// @Failure 404 {object} response.ErrorEnvelope "Auth session не найдена"
// @Failure 409 {object} response.ErrorEnvelope "Сессия ещё не готова, уже обменяна, уже использована или истекла"
// @Router /auth/max/exchange [post]
func (h *AuthHandler) ExchangeMAXAuth(w http.ResponseWriter, r *http.Request) {
	var req ExchangeMAXAuthRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid json body")
		return
	}

	session, err := h.service.ExchangeMAXAuth(r.Context(), req.SessionID)
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), authErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, session)
}

type UpdateMeRequest struct {
	FullName *string `json:"full_name"`
	Phone    *string `json:"phone"`
}

// UpdateMe godoc
// @Summary Обновить профиль текущего пользователя
// @Description Обновляет профиль пользователя, который определяется из Bearer JWT.
// @Description Можно передать full_name, phone или оба поля сразу.
// @Description Телефон нормализуется backend-логикой к цифровому виду без знака плюс и разделителей.
// @Description Хотя бы одно поле должно быть передано, иначе метод вернёт ошибку валидации.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateMeRequest true "Тело запроса с обновлением full_name и/или phone"
// @Success 200 {object} response.SuccessEnvelope{data=core.User} "Профиль успешно обновлён"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный JSON или ошибка валидации полей"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 404 {object} response.ErrorEnvelope "Пользователь не найден"
// @Failure 409 {object} response.ErrorEnvelope "Конфликт при обновлении профиля"
// @Router /me [patch]
func (h *AuthHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Failure(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req UpdateMeRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "invalid json body")
		return
	}

	user, err := h.service.UpdateProfile(r.Context(), userID, service.UpdateProfileInput{
		FullName: req.FullName,
		Phone:    req.Phone,
	})
	if err != nil {
		response.Failure(w, errorsstatus.HTTPStatus(err), authErrorMessage(err))
		return
	}

	response.Success(w, http.StatusOK, user)
}

func authErrorMessage(err error) string {
	switch {
	case errors.Is(err, errorsstatus.ErrInvalidInput):
		return "invalid input"
	case errors.Is(err, errorsstatus.ErrUnauthorized):
		switch {
		case errors.Is(err, service.ErrMAXInitDataExpired):
			return "max init data expired"
		case errors.Is(err, service.ErrMAXStartParamMismatch):
			return "max start param mismatch"
		default:
			return "unauthorized"
		}
	case errors.Is(err, errorsstatus.ErrConflict):
		switch {
		case errors.Is(err, service.ErrMAXAuthSessionNotReady):
			return "auth session not ready"
		case errors.Is(err, service.ErrMAXAuthSessionExpired):
			return "auth session expired"
		case errors.Is(err, service.ErrMAXAuthSessionAlreadyExchanged):
			return "auth session already exchanged"
		case errors.Is(err, service.ErrMAXAuthSessionAlreadyUsed):
			return "auth session already completed"
		default:
			return "conflict"
		}
	case errors.Is(err, errorsstatus.ErrNotFound):
		return "not found"
	case errors.Is(err, errorsstatus.ErrServiceUnavailable):
		return "max auth is unavailable"
	default:
		return "internal server error"
	}
}
