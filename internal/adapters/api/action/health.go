package action

import (
	"context"
	"net/http"
	"time"

	"eventAI/internal/adapters/api/response"

	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthHandler struct {
	db *pgxpool.Pool
}

func NewHealthHandler(db *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Get(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	databaseStatus := "ok"

	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		status = http.StatusServiceUnavailable
		databaseStatus = "unavailable"
	}

	response.JSON(w, status, "data", map[string]string{
		"status":   "ok",
		"database": databaseStatus,
	})
}
