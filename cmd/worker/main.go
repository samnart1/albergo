package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/samnart1/albergo/internal/adapter/notify"
	"github.com/samnart1/albergo/internal/adapter/postgres"
	"github.com/samnart1/albergo/internal/platform/config"
	"github.com/samnart1/albergo/internal/platform/db"
	"github.com/samnart1/albergo/internal/platform/logging"
	"github.com/samnart1/albergo/internal/worker"
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

	log := logging.New(cfg.LogLevel, cfg.Env).With("service", "worker")

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// worker doesn't migrate
	dispatcher := worker.NewDispatcher(
		postgres.NewOutboxRepository(pool),
		notify.NewLogSender(log),
		log,
		cfg.OutboxInterval,
		cfg.OutboxBatchSize,
	)

	log.Info("outbox dispatcher has started", "interval", cfg.OutboxInterval, "batch", cfg.OutboxBatchSize)
	return dispatcher.Run(ctx)
}
