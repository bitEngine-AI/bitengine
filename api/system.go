package api

import (
	"encoding/json"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/bitEngine-AI/bitengine/internal/monitor"
)

const Version = "0.1.0-mvp"

type SystemHandler struct {
	DB          *sqlx.DB
	RDB         *redis.Client
	CodegenMode string
}

func (h *SystemHandler) Status(w http.ResponseWriter, r *http.Request) {
	dbStatus := "connected"
	if err := h.DB.PingContext(r.Context()); err != nil {
		dbStatus = "disconnected"
	}

	redisStatus := "connected"
	if err := h.RDB.Ping(r.Context()).Err(); err != nil {
		redisStatus = "disconnected"
	}

	status := "ok"
	if dbStatus != "connected" || redisStatus != "connected" {
		status = "degraded"
	}

	resp := map[string]string{
		"status":       status,
		"version":      Version,
		"db":           dbStatus,
		"redis":        redisStatus,
		"codegen_mode": h.CodegenMode,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *SystemHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := monitor.Collect(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]any{
				"code":    "METRICS_ERROR",
				"message": err.Error(),
			},
		})
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}
