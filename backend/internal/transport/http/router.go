// Package http is the JSON API (v1) over net/http with Go 1.22+ routing patterns.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/service"
)

const (
	healthCheckTimeout = 2 * time.Second
	maxBodyBytes       = 1 << 20 // 1 MiB JSON
	maxCSVBytes        = 5 << 20
)

// RouterConfig wires the router.
type RouterConfig struct {
	AppEnv    string
	Service   *service.Service
	Assistant *service.Assistant
	Logger    *slog.Logger
	// Static serves the PWA at /app/ when non-nil.
	Static http.Handler
}

type api struct {
	cfg RouterConfig
	svc *service.Service
	ast *service.Assistant
	log *slog.Logger
}

// NewRouter builds the HTTP handler.
func NewRouter(cfg RouterConfig) http.Handler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	a := &api{cfg: cfg, svc: cfg.Service, ast: cfg.Assistant, log: cfg.Logger}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("GET /ready", a.ready)

	auth := func(pattern string, h func(w http.ResponseWriter, r *http.Request, act service.Actor)) {
		mux.HandleFunc(pattern, a.withAuth(h))
	}
	auth("GET /v1/me", a.me)

	auth("GET /v1/wines", a.listWines)
	auth("POST /v1/wines", a.createWine)
	auth("GET /v1/wines/{id}", a.getWine)
	auth("PATCH /v1/wines/{id}", a.patchWine)
	auth("DELETE /v1/wines/{id}", a.deleteWine)

	auth("GET /v1/cellar/items", a.listItems)
	auth("POST /v1/cellar/items", a.createItem)
	auth("GET /v1/cellar/items/{id}", a.getItem)
	auth("PATCH /v1/cellar/items/{id}", a.patchItem)
	auth("POST /v1/cellar/items/{id}/drink", a.drink)
	auth("GET /v1/cellar/items/{id}/recipes", a.recipesForItem)
	auth("GET /v1/cellar/summary", a.summary)
	auth("GET /v1/cellar/history", a.history)
	auth("GET /v1/cellar/buy-again", a.buyAgain)
	auth("GET /v1/reminders", a.reminders)

	auth("GET /v1/taste/survey/next", a.surveyNext)
	auth("POST /v1/taste/survey/answers", a.surveyAnswers)
	auth("GET /v1/taste/profile", a.profile)

	auth("GET /v1/recipes", a.listRecipes)
	auth("POST /v1/recipes", a.createRecipe)
	auth("GET /v1/recipes/{id}", a.getRecipe)
	auth("PATCH /v1/recipes/{id}", a.patchRecipe)
	auth("DELETE /v1/recipes/{id}", a.deleteRecipe)
	auth("GET /v1/recipes/{id}/wines", a.winesForRecipe)
	auth("POST /v1/recipes/{id}/feedback", a.pairingFeedback)

	auth("POST /v1/chat", a.chat)
	auth("GET /v1/export.csv", a.exportCSV)
	auth("POST /v1/import.csv", a.importCSV)
	auth("POST /v1/import.md", a.importMarkdown)

	if cfg.Static != nil {
		mux.Handle("GET /app/", http.StripPrefix("/app", cfg.Static))
		mux.HandleFunc("GET /app", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
		})
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	})
	return a.logRequests(mux)
}

// ---------- infrastructure ----------

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// logRequests logs method, route and status — never bodies, tokens or query values (may contain personal data).
func (a *api) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if rec := recover(); rec != nil {
				a.log.Error("panic in handler", slog.String("path", r.URL.Path), slog.Any("panic", rec))
				writeError(sw, http.StatusInternalServerError, "internal", "internal error")
			}
			a.log.Info("http", slog.String("method", r.Method), slog.String("path", r.URL.Path),
				slog.Int("status", sw.status), slog.Duration("took", time.Since(start)))
		}()
		next.ServeHTTP(sw, r)
	})
}

func (a *api) withAuth(h func(http.ResponseWriter, *http.Request, service.Actor)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if v := r.Header.Get("Authorization"); strings.HasPrefix(v, "Bearer ") {
			token = strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))
		}
		act, err := a.svc.Authenticate(r.Context(), token)
		if err != nil {
			a.fail(w, err)
			return
		}
		if l := r.URL.Query().Get("lang"); l != "" {
			act.User.Locale = domain.ParseLocale(l)
		}
		h(w, r, act)
	}
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field,omitempty"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	var b errorBody
	b.Error.Code, b.Error.Message = code, msg
	writeJSON(w, status, b)
}

// fail maps domain errors to HTTP. Unknown errors become 500 without leaking details.
func (a *api) fail(w http.ResponseWriter, err error) {
	var v *domain.ValidationError
	switch {
	case errors.As(err, &v):
		var b errorBody
		b.Error.Code, b.Error.Message, b.Error.Field = "validation_error", v.Error(), v.Field
		writeJSON(w, http.StatusBadRequest, b)
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "resource is in use or already exists")
	case errors.Is(err, domain.ErrOutOfStock):
		writeError(w, http.StatusConflict, "out_of_stock", "no bottles left")
	case errors.Is(err, domain.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid bearer token")
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "user has no cellar")
	case errors.Is(err, domain.ErrBudget):
		writeError(w, http.StatusTooManyRequests, "llm_budget_exceeded", "monthly LLM budget exceeded")
	case errors.Is(err, context.Canceled):
		writeError(w, 499, "canceled", "request canceled")
	default:
		a.log.Error("internal error", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal", "internal error")
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// decode reads a JSON body strictly (unknown fields are rejected — catches typos early).
func decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return domain.Invalid("body", "empty request body")
		}
		return domain.Invalid("body", "invalid JSON: %v", err)
	}
	return nil
}

func queryInt(r *http.Request, key string, def int) int {
	if v, err := strconv.Atoi(r.URL.Query().Get(key)); err == nil {
		return v
	}
	return def
}

type list[T any] struct {
	Items []T `json:"items"`
	Count int `json:"count"`
}

func newList[T any](xs []T) list[T] {
	if xs == nil {
		xs = []T{}
	}
	return list[T]{Items: xs, Count: len(xs)}
}

// ---------- health ----------

type healthResponse struct {
	Status   string `json:"status"`
	Env      string `json:"env"`
	Storage  string `json:"storage"`
	LLM      string `json:"llm_provider"`
	LLMModel string `json:"llm_model,omitempty"`
	Database string `json:"database,omitempty"`
}

func (a *api) healthBody() healthResponse {
	h := healthResponse{Status: "ok", Env: a.cfg.AppEnv, Storage: a.svc.Store().Kind()}
	if a.ast != nil {
		h.LLM, h.LLMModel = a.ast.Provider().Name(), a.ast.Provider().Model()
	}
	return h
}

func (a *api) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.healthBody())
}

func (a *api) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
	defer cancel()
	h := a.healthBody()
	if err := a.svc.Store().Ping(ctx); err != nil {
		h.Status, h.Database = "error", "unavailable"
		writeJSON(w, http.StatusServiceUnavailable, h)
		return
	}
	h.Database = "ok"
	writeJSON(w, http.StatusOK, h)
}
