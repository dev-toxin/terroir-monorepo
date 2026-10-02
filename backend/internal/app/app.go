// Package app wires config, storage, services, LLM and HTTP together.
// Used by cmd/api, cmd/seed and end-to-end tests.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/anko-soft-lab/terroir/backend/internal/config"
	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/llm"
	"github.com/anko-soft-lab/terroir/backend/internal/seed"
	"github.com/anko-soft-lab/terroir/backend/internal/service"
	"github.com/anko-soft-lab/terroir/backend/internal/storage"
	"github.com/anko-soft-lab/terroir/backend/internal/storage/sqlstore"
	terroirhttp "github.com/anko-soft-lab/terroir/backend/internal/transport/http"
	"github.com/anko-soft-lab/terroir/backend/internal/webapp"
)

// App is a fully wired application.
type App struct {
	Store     storage.Store
	Service   *service.Service
	Assistant *service.Assistant
	Handler   http.Handler
	Clock     domain.Clock
}

// OpenStore opens the configured storage and applies migrations.
func OpenStore(ctx context.Context, cfg config.Config) (storage.Store, error) {
	switch cfg.Storage {
	case config.StoragePostgres:
		return sqlstore.OpenPostgres(ctx, cfg.DatabaseURL)
	case config.StorageMemory:
		return sqlstore.OpenSQLite(ctx, ":memory:")
	default:
		return sqlstore.OpenSQLite(ctx, cfg.SQLitePath)
	}
}

// NewProvider creates the configured LLM provider.
func NewProvider(cfg config.LLM) llm.Provider {
	if cfg.Provider == "openai_compat" {
		return llm.NewOpenAICompat(llm.CompatConfig{
			BaseURL: cfg.BaseURL, Model: cfg.Model, APIKey: cfg.APIKey, AuthScheme: cfg.AuthScheme,
			ExtraHeaders: cfg.ExtraHeaders, ReasoningEffort: cfg.ReasoningEffort, Temperature: cfg.Temperature,
			MaxTokens: cfg.MaxTokens, Timeout: cfg.Timeout, MaxRetries: cfg.MaxRetries,
		})
	}
	return llm.Mock{}
}

// New builds the app over an opened store.
func New(ctx context.Context, cfg config.Config, st storage.Store, log *slog.Logger) (*App, error) {
	var clock domain.Clock = domain.SystemClock{}
	if cfg.FixedDate != nil {
		clock = domain.FixedClock{T: *cfg.FixedDate}
	}
	svc := service.New(st, clock, sqlstore.NewID)
	if cfg.AutoSeed {
		if err := seed.Run(ctx, st, clock.Now()); err != nil {
			return nil, fmt.Errorf("seed: %w", err)
		}
	}
	if err := svc.RegisterTokens(ctx, cfg.AuthTokens); err != nil {
		return nil, fmt.Errorf("AUTH_TOKENS: %w", err)
	}
	budget := &llm.Budget{MonthlyTokens: cfg.LLM.MonthlyTokens, MonthlyMicros: cfg.LLM.MonthlyBudget,
		PricePer1KIn: cfg.LLM.PricePer1KIn, PricePer1KOut: cfg.LLM.PricePer1KOut, Store: st, Now: clock.Now}
	ast := service.NewAssistant(svc, NewProvider(cfg.LLM), budget, cfg.LLM.ContextTokens, log)
	h := terroirhttp.NewRouter(terroirhttp.RouterConfig{AppEnv: cfg.AppEnv, Service: svc, Assistant: ast, Logger: log, Static: webapp.Handler()})
	return &App{Store: st, Service: svc, Assistant: ast, Handler: h, Clock: clock}, nil
}
