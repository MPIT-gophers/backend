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
// @Summary Parse raw text into wishlist
// @Description Uses AI to parse freeform text into wishlist and anti-wishlist items. Generates a new wishlist for the event. Only organizers can call this.
// @Tags wishlist
// @Accept json
// @Produce json
// @Param eventID path string true "Event UUID"
// @Param request body ParseWishlistRequest true "Text payload"
// @Success 200 {object} map[string]interface{} "items and anti_items arrays"
// @Failure 400 {object} response.ErrorEnvelope "Bad request"
// @Failure 401 {object} response.ErrorEnvelope "Unauthorized"
// @Failure 500 {object} response.ErrorEnvelope "Internal server error"
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
// @Summary Get event wishlist
// @Description Retrieves the wishlist items for the event. Guests see masked details (booked by others is hidden), organizers see everything.
// @Tags wishlist
// @Produce json
// @Param eventID path string true "Event UUID"
// @Success 200 {object} map[string]interface{} "items array"
// @Failure 400 {object} response.ErrorEnvelope "Invalid event ID format"
// @Failure 401 {object} response.ErrorEnvelope "Unauthorized"
// @Failure 500 {object} response.ErrorEnvelope "Internal server error"
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
// @Summary Submit a guest gift idea
// @Description Validates guest's text against the anti-wishlist using AI. Returns whether allowed or blocked, along with the reasoning.
// @Tags wishlist
// @Accept json
// @Produce json
// @Param eventID path string true "Event UUID"
// @Param request body SubmitGuestIdeaRequest true "Idea text payload"
// @Success 200 {object} map[string]interface{} "allowed, reason, item"
// @Failure 400 {object} response.ErrorEnvelope "Invalid request"
// @Failure 401 {object} response.ErrorEnvelope "Unauthorized"
// @Failure 500 {object} response.ErrorEnvelope "Failed to submit idea"
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
// @Summary Book a wishlist item
// @Description Allows a guest to book an entire item so others cannot fund or book it. Returns HTTP 409 Conflict if already booked.
// @Tags wishlist
// @Produce json
// @Param eventID path string true "Event UUID"
// @Param itemID path string true "Item UUID"
// @Success 200 {object} map[string]interface{} "message"
// @Failure 400 {object} response.ErrorEnvelope "Invalid UUID"
// @Failure 401 {object} response.ErrorEnvelope "Unauthorized"
// @Failure 409 {object} response.ErrorEnvelope "Item already booked or has funds"
// @Failure 500 {object} response.ErrorEnvelope "Internal Server Error"
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
// @Summary Fund a wishlist item partially
// @Description Allows a guest to crowdsource funds for an item. Returns HTTP 409 if the item is already fully booked.
// @Tags wishlist
// @Accept json
// @Produce json
// @Param eventID path string true "Event UUID"
// @Param itemID path string true "Item UUID"
// @Param request body FundItemRequest true "Amount payload"
// @Success 200 {object} map[string]interface{} "current_fund amount"
// @Failure 400 {object} response.ErrorEnvelope "Invalid payload or UUID"
// @Failure 401 {object} response.ErrorEnvelope "Unauthorized"
// @Failure 409 {object} response.ErrorEnvelope "Item is already booked"
// @Failure 500 {object} response.ErrorEnvelope "Internal Server Error"
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
