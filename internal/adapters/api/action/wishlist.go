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
