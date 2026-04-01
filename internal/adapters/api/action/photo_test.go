package action

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"eventAI/internal/adapters/api/middleware"
	errorsstatus "eventAI/internal/errorsStatus"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type mockPhotoService struct {
	err error
}

func (m *mockPhotoService) UploadPhoto(ctx context.Context, eventID uuid.UUID, guestID uuid.UUID, fileBytes []byte, mimeType string) error {
	return m.err
}

func (m *mockPhotoService) GetEventPhotos(ctx context.Context, eventID uuid.UUID, userID uuid.UUID) ([]string, error) {
	return nil, m.err
}

func TestPhotoHandler_UploadPhotos(t *testing.T) {
	t.Parallel()

	validEventID := uuid.New().String()
	validUserID := uuid.New().String()

	tests := []struct {
		name           string
		eventIDParam   string
		setupContext   func(r *http.Request) *http.Request
		setupMultipart func() (*bytes.Buffer, string)
		expectedStatus int
	}{
		{
			name:         "Success 1 photo",
			eventIDParam: validEventID,
			setupContext: func(r *http.Request) *http.Request {
				ctx := middleware.WithUserID(r.Context(), validUserID)
				return r.WithContext(ctx)
			},
			setupMultipart: func() (*bytes.Buffer, string) {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				part, _ := writer.CreateFormFile("photos", "test.jpg")
				part.Write([]byte("fake image content"))
				writer.Close()
				return body, writer.FormDataContentType()
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:         "More than 10 files",
			eventIDParam: validEventID,
			setupContext: func(r *http.Request) *http.Request {
				ctx := middleware.WithUserID(r.Context(), validUserID)
				return r.WithContext(ctx)
			},
			setupMultipart: func() (*bytes.Buffer, string) {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				for i := 0; i < 11; i++ {
					part, _ := writer.CreateFormFile("photos", "test.jpg")
					part.Write([]byte("fake image content"))
				}
				writer.Close()
				return body, writer.FormDataContentType()
			},
			expectedStatus: http.StatusBadRequest, // "maximum 10 photos per batch allowed"
		},
		{
			name:         "Missing photos array",
			eventIDParam: validEventID,
			setupContext: func(r *http.Request) *http.Request {
				ctx := middleware.WithUserID(r.Context(), validUserID)
				return r.WithContext(ctx)
			},
			setupMultipart: func() (*bytes.Buffer, string) {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				// Create a random field instead
				writer.WriteField("random_field", "123")
				writer.Close()
				return body, writer.FormDataContentType()
			},
			expectedStatus: http.StatusBadRequest, // "no photos provided in 'photos' field"
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockPhotoService{}
			handler := NewPhotoHandler(svc)

			body, contentType := tc.setupMultipart()
			req := httptest.NewRequest(http.MethodPost, "/upload", body)
			req.Header.Set("Content-Type", contentType)

			if tc.setupContext != nil {
				req = tc.setupContext(req)
			}

			// Add chi URL params
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("eventID", tc.eventIDParam)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler.UploadPhotos(rr, req)

			if rr.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tc.expectedStatus, rr.Code, rr.Body.String())
			}
			
			if tc.expectedStatus == http.StatusBadRequest {
				if !strings.Contains(rr.Body.String(), "error") {
					t.Errorf("expected error message in body but got %s", rr.Body.String())
				}
			}
		})
	}
}

func TestPhotoHandler_GetPhotos(t *testing.T) {
	t.Parallel()

	validEventID := uuid.New()
	validUserID := uuid.New()

	tests := []struct {
		name         string
		eventID      string
		serviceErr   error
		serviceUrls  []string
		wantStatus   int
	}{
		{
			name:        "Success returns 200",
			eventID:     validEventID.String(),
			serviceUrls: []string{"/1.jpg", "/2.jpg"},
			wantStatus:  http.StatusOK,
		},
		{
			name:        "Forbidden returns 403",
			eventID:     validEventID.String(),
			serviceErr:  errorsstatus.ErrForbidden,
			wantStatus:  http.StatusForbidden,
		},
		{
			name:        "Invalid Event ID returns 400",
			eventID:     "invalid-uuid",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockSvc := &mockPhotoService{err: tc.serviceErr}
			handler := NewPhotoHandler(mockSvc)

			r := chi.NewRouter()
			// Mock middleware by injecting UserID 
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := r.Context()
					if tc.eventID != "invalid-uuid" {
						ctx = middleware.WithUserID(ctx, validUserID.String())
					}
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})

			r.Get("/events/{eventID}/photos", handler.GetPhotos)

			url := "/events/" + tc.eventID + "/photos"
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d. Body: %s", tc.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}
