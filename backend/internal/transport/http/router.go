package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	healthPath         = "/health"
	readinessPath      = "/ready"
	jsonContentType    = "application/json"
	contentTypeHeader  = "Content-Type"
	healthCheckTimeout = 2 * time.Second
)

type RouterConfig struct {
	AppEnv string
	DB     *pgxpool.Pool
}

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database,omitempty"`
	Env      string `json:"env,omitempty"`
}

func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Env: cfg.AppEnv})
	})

	mux.HandleFunc(readinessPath, func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := contextWithTimeout(r, healthCheckTimeout)
		defer cancel()

		if err := cfg.DB.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "error", Database: "unavailable"})
			return
		}

		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Database: "ok", Env: cfg.AppEnv})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set(contentTypeHeader, jsonContentType)
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func contextWithTimeout(r *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), timeout)
}
