package action

import (
	"encoding/json"
	"net/http"

	apimiddleware "eventAI/internal/adapters/api/middleware"
	"eventAI/internal/adapters/api/response"
	"eventAI/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type WishlistHandler struct {
	wishlistService service.WishlistService
}

func NewWishlistHandler(s service.WishlistService) *WishlistHandler {
	return &WishlistHandler{
		wishlistService: s,
	}
}

type ParseWishlistRequest struct {
	Text string `json:"text"`
}

// ParseWishlist godoc
// @Summary Разобрать свободный текст в wishlist и anti-wishlist
// @Description Принимает произвольный текст и с помощью AI разбирает его на список желаемых подарков и anti-wishlist элементов.
// @Description Обычно используется организатором события для первичного наполнения wishlist по заметкам, сообщениям или черновому описанию.
// @Description В ответе возвращаются массивы items и anti_items, сформированные на основе текста.
// @Tags wishlist
// @Accept json
// @Produce json
// @Param eventID path string true "Идентификатор события"
// @Param request body ParseWishlistRequest true "Тело запроса со свободным текстом для разбора"
// @Success 200 {object} map[string]interface{} "Сформированные массивы items и anti_items"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный eventID, JSON или пустой текст"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 500 {object} response.ErrorEnvelope "Внутренняя ошибка при AI-разборе wishlist"
// @Router /api/v1/events/{eventID}/wishlist/parse [post]
func (h *WishlistHandler) ParseWishlist(w http.ResponseWriter, r *http.Request) {
	eventIDStr := chi.URLParam(r, "eventID")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		response.Failure(w, http.StatusBadRequest, "Invalid event ID format")
		return
	}

	var req ParseWishlistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Text == "" {
		response.Failure(w, http.StatusBadRequest, "Text is required")
		return
	}

	items, antiItems, err := h.wishlistService.ParseWishlist(r.Context(), eventID, req.Text)
	if err != nil {
		response.Failure(w, http.StatusInternalServerError, "Failed to parse wishlist")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"items":      items,
		"anti_items": antiItems,
	})
}

// GetWishlist godoc
// @Summary Получить wishlist события
// @Description Возвращает список wishlist-позиций, привязанных к событию.
// @Description Содержимое ответа может зависеть от роли пользователя: организатор видит больше деталей, гостям часть информации может быть скрыта.
// @Description Метод используется для отображения списка подарков и текущего состояния бронирования или финансирования.
// @Tags wishlist
// @Produce json
// @Param eventID path string true "Идентификатор события"
// @Success 200 {object} map[string]interface{} "Массив wishlist-позиций события"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный eventID"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 500 {object} response.ErrorEnvelope "Внутренняя ошибка при получении wishlist"
// @Router /api/v1/events/{eventID}/wishlist [get]
func (h *WishlistHandler) GetWishlist(w http.ResponseWriter, r *http.Request) {
	eventIDStr := chi.URLParam(r, "eventID")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		response.Failure(w, http.StatusBadRequest, "Invalid event ID format")
		return
	}

	userIDStr, ok := apimiddleware.UserIDFromContext(r.Context())
	if !ok {
		response.Failure(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, "invalid user id")
		return
	}

	items, err := h.wishlistService.GetWishlistForUser(r.Context(), userID, eventID)
	if err != nil {
		response.Failure(w, http.StatusInternalServerError, "Failed to get wishlist")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"items": items,
	})
}

type SubmitGuestIdeaRequest struct {
	Text string `json:"text"`
}

// SubmitGuestIdea godoc
// @Summary Отправить идею подарка от гостя
// @Description Принимает текстовое предложение гостя и проверяет его на соответствие anti-wishlist ограничениям.
// @Description В ответе backend возвращает, разрешена ли идея, а если нет — причину блокировки.
// @Description Если идея допустима, в ответ также может быть возвращён созданный или подобранный item.
// @Tags wishlist
// @Accept json
// @Produce json
// @Param eventID path string true "Идентификатор события"
// @Param request body SubmitGuestIdeaRequest true "Тело запроса с текстом идеи подарка"
// @Success 200 {object} map[string]interface{} "Результат проверки идеи: allowed, reason, item"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный eventID, JSON или пустой текст"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 500 {object} response.ErrorEnvelope "Внутренняя ошибка при обработке идеи"
// @Router /api/v1/events/{eventID}/wishlist/ideas [post]
func (h *WishlistHandler) SubmitGuestIdea(w http.ResponseWriter, r *http.Request) {
	eventIDStr := chi.URLParam(r, "eventID")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		response.Failure(w, http.StatusBadRequest, "Invalid event ID format")
		return
	}

	userIDStr, ok := apimiddleware.UserIDFromContext(r.Context())
	if !ok {
		response.Failure(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, "invalid user id")
		return
	}

	var req SubmitGuestIdeaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Text == "" {
		response.Failure(w, http.StatusBadRequest, "Text is required")
		return
	}

	allowed, reason, item, err := h.wishlistService.SubmitGuestIdea(r.Context(), eventID, userID, req.Text)
	if err != nil {
		response.Failure(w, http.StatusInternalServerError, "Failed to submit idea")
		return
	}

	respData := map[string]interface{}{
		"allowed": allowed,
	}
	if !allowed {
		respData["reason"] = reason
	} else {
		respData["item"] = item
	}

	response.Success(w, http.StatusOK, respData)
}

// BookItem godoc
// @Summary Забронировать wishlist item целиком
// @Description Позволяет пользователю забронировать конкретный подарок полностью.
// @Description После бронирования item нельзя частично финансировать или повторно бронировать другим участникам.
// @Description Если позиция уже занята или по ней уже есть сбор средств, backend вернёт конфликт.
// @Tags wishlist
// @Produce json
// @Param eventID path string true "Идентификатор события"
// @Param itemID path string true "Идентификатор wishlist item"
// @Success 200 {object} map[string]interface{} "Сообщение об успешном бронировании"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный eventID или itemID"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 409 {object} response.ErrorEnvelope "Item уже забронирован или по нему уже есть финансирование"
// @Failure 500 {object} response.ErrorEnvelope "Внутренняя ошибка при бронировании item"
// @Router /api/v1/events/{eventID}/wishlist/{itemID}/book [post]
func (h *WishlistHandler) BookItem(w http.ResponseWriter, r *http.Request) {
	eventIDStr := chi.URLParam(r, "eventID")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		response.Failure(w, http.StatusBadRequest, "Invalid event ID format")
		return
	}

	itemIDStr := chi.URLParam(r, "itemID")
	itemID, err := uuid.Parse(itemIDStr)
	if err != nil {
		response.Failure(w, http.StatusBadRequest, "Invalid item ID format")
		return
	}

	userIDStr, ok := apimiddleware.UserIDFromContext(r.Context())
	if !ok {
		response.Failure(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, "invalid user id")
		return
	}

	err = h.wishlistService.BookItem(r.Context(), eventID, itemID, userID)
	if err != nil {
		if err.Error() == "item already booked" || err.Error() == "item already has funds, cannot book entirely" {
			response.Failure(w, http.StatusConflict, err.Error())
			return
		}
		response.Failure(w, http.StatusInternalServerError, "Failed to book item: "+err.Error())
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Item successfully booked",
	})
}

type FundItemRequest struct {
	Amount float64 `json:"amount"`
}

// FundItem godoc
// @Summary Частично профинансировать wishlist item
// @Description Позволяет пользователю внести сумму в общий сбор средств на конкретный подарок.
// @Description Используется для сценария коллективного подарка, когда несколько гостей складываются на одну позицию.
// @Description Если item уже полностью забронирован, backend вернёт конфликт.
// @Tags wishlist
// @Accept json
// @Produce json
// @Param eventID path string true "Идентификатор события"
// @Param itemID path string true "Идентификатор wishlist item"
// @Param request body FundItemRequest true "Тело запроса с суммой пополнения"
// @Success 200 {object} map[string]interface{} "Текущее накопленное значение current_fund"
// @Failure 400 {object} response.ErrorEnvelope "Некорректный eventID, itemID, JSON или сумма"
// @Failure 401 {object} response.ErrorEnvelope "JWT отсутствует или невалиден"
// @Failure 409 {object} response.ErrorEnvelope "Item уже полностью забронирован"
// @Failure 500 {object} response.ErrorEnvelope "Внутренняя ошибка при пополнении сбора"
// @Router /api/v1/events/{eventID}/wishlist/{itemID}/fund [post]
func (h *WishlistHandler) FundItem(w http.ResponseWriter, r *http.Request) {
	eventIDStr := chi.URLParam(r, "eventID")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		response.Failure(w, http.StatusBadRequest, "Invalid event ID format")
		return
	}

	itemIDStr := chi.URLParam(r, "itemID")
	itemID, err := uuid.Parse(itemIDStr)
	if err != nil {
		response.Failure(w, http.StatusBadRequest, "Invalid item ID format")
		return
	}

	userIDStr, ok := apimiddleware.UserIDFromContext(r.Context())
	if !ok {
		response.Failure(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Failure(w, http.StatusUnauthorized, "invalid user id")
		return
	}

	var req FundItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Failure(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Amount <= 0 {
		response.Failure(w, http.StatusBadRequest, "Amount must be greater than zero")
		return
	}

	newFund, err := h.wishlistService.FundItem(r.Context(), eventID, itemID, userID, req.Amount)
	if err != nil {
		if err.Error() == "item is already booked" {
			response.Failure(w, http.StatusConflict, err.Error())
			return
		}
		response.Failure(w, http.StatusInternalServerError, "Failed to fund item: "+err.Error())
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"current_fund": newFund,
	})
}
