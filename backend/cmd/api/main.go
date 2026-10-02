// Command api runs the Terroir HTTP API.
//
//	go run ./cmd/api            # serve
//	go run ./cmd/api -seed      # seed demo household and exit
//	go run ./cmd/api -reset     # delete the local SQLite file, then continue (dev only)
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anko-soft-lab/terroir/backend/internal/app"
	"github.com/anko-soft-lab/terroir/backend/internal/config"
	"github.com/anko-soft-lab/terroir/backend/internal/seed"
)

const (
	startupTimeout  = 30 * time.Second
	shutdownTimeout = 10 * time.Second
)

func main() {
	seedOnly := flag.Bool("seed", false, "seed demo data and exit")
	reset := flag.Bool("reset", false, "delete the local SQLite database file before start (STORAGE=sqlite only)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger, *seedOnly, *reset); err != nil {
		logger.Error("fatal", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(logger *slog.Logger, seedOnly, reset bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if reset {
		if cfg.Storage != config.StorageSQLite {
			return errors.New("-reset is only supported for STORAGE=sqlite")
		}
		for _, suffix := range []string{"", "-wal", "-shm"} {
			if err := os.Remove(cfg.SQLitePath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		logger.Info("local database removed", slog.String("path", cfg.SQLitePath))
	}

	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	defer cancel()
	st, err := app.OpenStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	if seedOnly {
		a, err := app.New(ctx, cfg, st, logger)
		if err != nil {
			return err
		}
		if err := seed.Run(ctx, st, a.Clock.Now()); err != nil {
			return err
		}
		logger.Info("demo data seeded", slog.String("storage", st.Kind()),
			slog.String("tokens", seed.TokenAnton+", "+seed.TokenElena))
		return nil
	}

	a, err := app.New(ctx, cfg, st, logger)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: a.Handler, ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting api server", slog.String("addr", cfg.HTTPAddr), slog.String("env", cfg.AppEnv),
			slog.String("storage", st.Kind()), slog.String("llm_provider", a.Assistant.Provider().Name()))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
	case err := <-errCh:
		return err
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	logger.Info("shutting down api server")
	return server.Shutdown(shutdownCtx)
}
