package action

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocsHandlerRedirect(t *testing.T) {
	t.Parallel()

	handler := NewDocsHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	rec := httptest.NewRecorder()

	handler.Redirect(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMovedPermanently)
	}

	if got := rec.Header().Get("Location"); got != "/api/docs/" {
		t.Fatalf("location = %q, want /api/docs/", got)
	}
}

func TestDocsHandlerIndex(t *testing.T) {
	t.Parallel()

	handler := NewDocsHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/docs/", nil)
	rec := httptest.NewRecorder()

	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("content-type = %q, want text/html", got)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "swagger.json") {
		t.Fatalf("body does not reference swagger.json")
	}
}
