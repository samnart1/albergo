package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/samnart1/albergo/internal/adapter/postgres"
	"github.com/samnart1/albergo/internal/adapter/rest"
	"github.com/samnart1/albergo/internal/app"
	"github.com/samnart1/albergo/internal/domain/shared"
	"github.com/samnart1/albergo/internal/platform/config"
	"github.com/samnart1/albergo/internal/platform/db"
	"github.com/samnart1/albergo/internal/platform/logging"
	"github.com/samnart1/albergo/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("shutting down", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logging.New(cfg.LogLevel, cfg.Env)

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if cfg.AutoMigrate {
		if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
			return err
		}
		log.Info("migrations applied")
	}

	services := app.New(app.Deps{
		Tx:           postgres.NewTxManager(pool),
		Catalog:      postgres.NewCatalogRepository(pool),
		Rates:        postgres.NewRateRepository(pool),
		Inventory:    postgres.NewInventoryRepository(pool),
		Guests:       postgres.NewGuestRepository(pool),
		Reservations: postgres.NewReservationRepository(pool),
		Clock:        shared.SystemClock{},
		Idempotency:  postgres.NewIdempotencyRepository(pool),
		Outbox:       postgres.NewOutboxRepository(pool),
	})

	handlers := rest.NewHandlers(services, log)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           rest.NewRouter(log, pool, handlers, cfg.StaffAPIKey),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Info("signal received, draining connections")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}
