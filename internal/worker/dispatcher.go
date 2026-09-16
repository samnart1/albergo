package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/samnart1/albergo/internal/app/ports"
)

type Dispatcher struct {
	outbox   ports.OutboxRepository
	sender   ports.Sender
	log      *slog.Logger
	interval time.Duration
	batch    int
}

func NewDispatcher(outbox ports.OutboxRepository, sender ports.Sender, log *slog.Logger, interval time.Duration, batch int) *Dispatcher {
	return &Dispatcher{outbox: outbox, sender: sender, log: log, interval: interval, batch: batch}
}

func (d *Dispatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		sent, err := d.outbox.Dispatch(ctx, d.batch, d.sender.Send)
		switch {
		case errors.Is(err, context.Canceled):
			return nil
		case err != nil:
			d.log.ErrorContext(ctx, "dispatch failed", "error", err)
		case sent > 0:
			d.log.InfoContext(ctx, "dispatched", "count", sent)
		}

		if err == nil && sent == d.batch {
			continue
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
