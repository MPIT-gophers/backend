package action

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	apimiddleware "eventAI/internal/adapters/api/middleware"
	"eventAI/internal/entities/core"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// stubWishlistService mocks the WishlistService
type stubWishlistService struct {
	items     []core.WishlistItem
	antiItems []core.AntiWishlistItem
	err       error
}

func (s *stubWishlistService) ParseWishlist(ctx context.Context, eventID uuid.UUID, text string) ([]core.WishlistItem, []core.AntiWishlistItem, error) {
	return s.items, s.antiItems, s.err
}

func (s *stubWishlistService) GetWishlist(ctx context.Context, eventID uuid.UUID) ([]core.WishlistItem, error) {
	return s.items, s.err
}

func (s *stubWishlistService) GetWishlistForUser(ctx context.Context, userID, eventID uuid.UUID) ([]core.WishlistItem, error) {
	return s.items, s.err
}

func (s *stubWishlistService) SubmitGuestIdea(ctx context.Context, eventID, userID uuid.UUID, ideaText string) (bool, string, *core.WishlistItem, error) {
	if ideaText == "bad" {
		return false, "blocked", nil, s.err
	}
	item := &core.WishlistItem{Name: ideaText}
	return true, "", item, s.err
}

func (s *stubWishlistService) BookItem(ctx context.Context, eventID, itemID, userID uuid.UUID) error {
	return s.err
}

func (s *stubWishlistService) FundItem(ctx context.Context, eventID, itemID, userID uuid.UUID, amount float64) (float64, error) {
	return amount, s.err
}


func TestWishlistHandler_ParseWishlist(t *testing.T) {
	t.Parallel()

	validEventID := uuid.New().String()

	tests := []struct {
		name           string
		eventIDParam   string
		requestBody    interface{}
		stubItems      []core.WishlistItem
		stubAntiItems  []core.AntiWishlistItem
		stubErr        error
		expectedStatus int
	}{
		{
			name:         "Success",
			eventIDParam: validEventID,
			requestBody:  ParseWishlistRequest{Text: "Хочу торт"},
			stubItems: []core.WishlistItem{
				{Name: "Торт"},
			},
			stubAntiItems:  []core.AntiWishlistItem{},
			stubErr:        nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid UUID",
			eventIDParam:   "not-uuid",
			requestBody:    ParseWishlistRequest{Text: "Хочу торт"},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid JSON",
			eventIDParam:   validEventID,
			requestBody:    "just string, not json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Empty Text",
			eventIDParam:   validEventID,
			requestBody:    ParseWishlistRequest{Text: ""},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Service Error",
			eventIDParam:   validEventID,
			requestBody:    ParseWishlistRequest{Text: "Хочу торт"},
			stubErr:        errors.New("db error"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubService := &stubWishlistService{
				items:     tc.stubItems,
				antiItems: tc.stubAntiItems,
				err:       tc.stubErr,
			}
			handler := NewWishlistHandler(stubService)

			var reqBodyBytes []byte
			var err error
			if s, ok := tc.requestBody.(string); ok {
				reqBodyBytes = []byte(s)
			} else {
				reqBodyBytes, err = json.Marshal(tc.requestBody)
				if err != nil {
					t.Fatalf("Failed to marshal request body: %v", err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/parse", bytes.NewBuffer(reqBodyBytes))
			
			// Use chi router to inject URL params
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("eventID", tc.eventIDParam)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler.ParseWishlist(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Fatalf("Expected status %d but got %d. Body: %s", tc.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestWishlistHandler_GetWishlist(t *testing.T) {
	t.Parallel()

	validEventID := uuid.New().String()
	validUserID := uuid.New().String()

	tests := []struct {
		name           string
		eventIDParam   string
		userIDCtx      string
		serviceItems   []core.WishlistItem
		serviceErr     error
		expectedStatus int
	}{
		{
			name:           "Success",
			eventIDParam:   validEventID,
			userIDCtx:      validUserID,
			serviceItems:   []core.WishlistItem{{Name: "Торт"}},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid Event UUID",
			eventIDParam:   "not-uuid",
			userIDCtx:      validUserID,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Unauthorized (No user ID)",
			eventIDParam:   validEventID,
			userIDCtx:      "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Service Error",
			eventIDParam:   validEventID,
			userIDCtx:      validUserID,
			serviceErr:     errors.New("role check failed"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubService := &stubWishlistService{
				items: tc.serviceItems,
				err:   tc.serviceErr,
			}
			handler := NewWishlistHandler(stubService)

			req := httptest.NewRequest(http.MethodGet, "/wishlist", nil)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("eventID", tc.eventIDParam)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			if tc.userIDCtx != "" {
				req = req.WithContext(apimiddleware.WithUserID(req.Context(), tc.userIDCtx))
			}

			rr := httptest.NewRecorder()
			handler.GetWishlist(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Fatalf("Expected status %d but got %d. Body: %s", tc.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestWishlistHandler_SubmitGuestIdea(t *testing.T) {
	t.Parallel()

	validEventID := uuid.New().String()
	validUserID := uuid.New().String()

	tests := []struct {
		name           string
		eventIDParam   string
		userIDCtx      string
		requestBody    interface{}
		serviceErr     error
		expectedStatus int
		expectAllowed  bool
	}{
		{
			name:           "Success Allowed",
			eventIDParam:   validEventID,
			userIDCtx:      validUserID,
			requestBody:    map[string]string{"text": "good"},
			expectedStatus: http.StatusOK,
			expectAllowed:  true,
		},
		{
			name:           "Success Blocked",
			eventIDParam:   validEventID,
			userIDCtx:      validUserID,
			requestBody:    map[string]string{"text": "bad"},
			expectedStatus: http.StatusOK,
			expectAllowed:  false,
		},
		{
			name:           "Invalid JSON",
			eventIDParam:   validEventID,
			userIDCtx:      validUserID,
			requestBody:    "not json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Empty Text",
			eventIDParam:   validEventID,
			userIDCtx:      validUserID,
			requestBody:    map[string]string{"text": ""},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Unauthorized",
			eventIDParam:   validEventID,
			userIDCtx:      "",
			requestBody:    map[string]string{"text": "good"},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Service Error",
			eventIDParam:   validEventID,
			userIDCtx:      validUserID,
			requestBody:    map[string]string{"text": "good"},
			serviceErr:     errors.New("db error"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubService := &stubWishlistService{
				err: tc.serviceErr,
			}
			handler := NewWishlistHandler(stubService)

			var reqBodyBytes []byte
			var err error
			if s, ok := tc.requestBody.(string); ok {
				reqBodyBytes = []byte(s)
			} else {
				reqBodyBytes, err = json.Marshal(tc.requestBody)
				if err != nil {
					t.Fatalf("Failed to marshal request body: %v", err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/ideas", bytes.NewBuffer(reqBodyBytes))
			
			// Chi context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("eventID", tc.eventIDParam)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Auth context
			if tc.userIDCtx != "" {
				req = req.WithContext(apimiddleware.WithUserID(req.Context(), tc.userIDCtx))
			}

			rr := httptest.NewRecorder()
			handler.SubmitGuestIdea(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Fatalf("Expected code %d got %d. Body: %s", tc.expectedStatus, rr.Code, rr.Body.String())
			}

			if rr.Code == http.StatusOK {
				var resp map[string]interface{}
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				
				data, ok := resp["data"].(map[string]interface{})
				if !ok {
					t.Fatalf("Response missing 'data' object")
				}

				allowed, ok := data["allowed"].(bool)
				if !ok || allowed != tc.expectAllowed {
					t.Errorf("Expected allowed %v got %v", tc.expectAllowed, allowed)
				}
			}
		})
	}
}

func TestWishlistHandler_BookItem(t *testing.T) {
	t.Parallel()

	validEventID := uuid.New().String()
	validItemID := uuid.New().String()
	validUserID := uuid.New().String()

	tests := []struct {
		name           string
		eventIDParam   string
		itemIDParam    string
		userIDCtx      string
		serviceErr     error
		expectedStatus int
	}{
		{
			name:           "Success",
			eventIDParam:   validEventID,
			itemIDParam:    validItemID,
			userIDCtx:      validUserID,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid Event UUID",
			eventIDParam:   "not-uuid",
			itemIDParam:    validItemID,
			userIDCtx:      validUserID,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid Item UUID",
			eventIDParam:   validEventID,
			itemIDParam:    "not-uuid",
			userIDCtx:      validUserID,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Unauthorized",
			eventIDParam:   validEventID,
			itemIDParam:    validItemID,
			userIDCtx:      "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Already booked conflict",
			eventIDParam:   validEventID,
			itemIDParam:    validItemID,
			userIDCtx:      validUserID,
			serviceErr:     errors.New("item already booked"),
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "Internal server error",
			eventIDParam:   validEventID,
			itemIDParam:    validItemID,
			userIDCtx:      validUserID,
			serviceErr:     errors.New("db error"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubService := &stubWishlistService{err: tc.serviceErr}
			handler := NewWishlistHandler(stubService)

			req := httptest.NewRequest(http.MethodPost, "/book", nil)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("eventID", tc.eventIDParam)
			rctx.URLParams.Add("itemID", tc.itemIDParam)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			if tc.userIDCtx != "" {
				req = req.WithContext(apimiddleware.WithUserID(req.Context(), tc.userIDCtx))
			}

			rr := httptest.NewRecorder()
			handler.BookItem(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Fatalf("Expected status %d got %d. Body: %s", tc.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestWishlistHandler_FundItem(t *testing.T) {
	t.Parallel()

	validEventID := uuid.New().String()
	validItemID := uuid.New().String()
	validUserID := uuid.New().String()

	tests := []struct {
		name           string
		eventIDParam   string
		itemIDParam    string
		userIDCtx      string
		requestBody    interface{}
		serviceErr     error
		expectedStatus int
	}{
		{
			name:           "Success",
			eventIDParam:   validEventID,
			itemIDParam:    validItemID,
			userIDCtx:      validUserID,
			requestBody:    FundItemRequest{Amount: 500.0},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid Amount (Zero)",
			eventIDParam:   validEventID,
			itemIDParam:    validItemID,
			userIDCtx:      validUserID,
			requestBody:    FundItemRequest{Amount: 0.0},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid Amount (Negative)",
			eventIDParam:   validEventID,
			itemIDParam:    validItemID,
			userIDCtx:      validUserID,
			requestBody:    FundItemRequest{Amount: -100.0},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid JSON",
			eventIDParam:   validEventID,
			itemIDParam:    validItemID,
			userIDCtx:      validUserID,
			requestBody:    "bad json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Item already booked conflict",
			eventIDParam:   validEventID,
			itemIDParam:    validItemID,
			userIDCtx:      validUserID,
			requestBody:    FundItemRequest{Amount: 50.0},
			serviceErr:     errors.New("item is already booked"),
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "Internal Error",
			eventIDParam:   validEventID,
			itemIDParam:    validItemID,
			userIDCtx:      validUserID,
			requestBody:    FundItemRequest{Amount: 50.0},
			serviceErr:     errors.New("db err"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubService := &stubWishlistService{err: tc.serviceErr}
			handler := NewWishlistHandler(stubService)

			var reqBodyBytes []byte
			var err error
			if s, ok := tc.requestBody.(string); ok {
				reqBodyBytes = []byte(s)
			} else {
				reqBodyBytes, err = json.Marshal(tc.requestBody)
				if err != nil {
					t.Fatalf("Failed to marshal: %v", err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/fund", bytes.NewBuffer(reqBodyBytes))

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("eventID", tc.eventIDParam)
			rctx.URLParams.Add("itemID", tc.itemIDParam)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			if tc.userIDCtx != "" {
				req = req.WithContext(apimiddleware.WithUserID(req.Context(), tc.userIDCtx))
			}

			rr := httptest.NewRecorder()
			handler.FundItem(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Fatalf("Expected status %d got %d. Body: %s", tc.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}
