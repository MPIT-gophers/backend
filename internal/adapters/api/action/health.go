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

type HealthData struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

func NewHealthHandler(db *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{db: db}
}

// Get godoc
// @Summary Проверить состояние сервиса
// @Description Выполняет базовую healthcheck-проверку backend-сервиса.
// @Description Endpoint дополнительно проверяет доступность PostgreSQL через Ping.
// @Description Если приложение живо, но база недоступна, метод вернёт 503 и укажет database=unavailable.
// @Tags health
// @Produce json
// @Success 200 {object} response.SuccessEnvelope{data=HealthData} "Сервис и база доступны"
// @Success 503 {object} response.SuccessEnvelope{data=HealthData} "Сервис отвечает, но база недоступна"
// @Router /healthz [get]
func (h *HealthHandler) Get(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	databaseStatus := "ok"

	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		status = http.StatusServiceUnavailable
		databaseStatus = "unavailable"
	}

	response.Success(w, status, HealthData{
		Status:   "ok",
		Database: databaseStatus,
	})
}
