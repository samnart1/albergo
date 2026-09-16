//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"uuid"

	"github.com/samnart1/albergo/internal/adapter/postgres"
	"github.com/samnart1/albergo/internal/app"
	"github.com/samnart1/albergo/internal/app/ports"
)

func TestOutboxLifecycle(t *testing.T) {
	srv, fx := newServer(t)
	publish(t, srv.URL, fx, 1)

	body := bookingBody(fx)
	key := uuid.New().String()

	status, raw := doWithKey(t, http.MethodPost, srv.URL+"/v1/reservations", body, key)
	if status != http.StatusCreated {
		t.Fatalf("create = %d %s", status, raw)
	}
	reference := decodeInto[struct {
		Reference string `json:"reference"`
	}](t, raw).Reference

	status, raw = doWithKey(t, http.MethodPost, srv.URL+"/v1/reservations", body, key)
	if status != http.StatusCreated {
		t.Fatalf("replay = %d %s", status, raw)
	}

	ctx := context.Background()
	outbox := postgres.NewOutboxRepository(pool)

	var pending int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM outbox WHERE published_at IS NULL`).Scan(&pending); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if pending != 1 {
		t.Fatalf("pending = %d, want 1", pending)
	}

	t.Run("a failing send is retried, not lost", func(t *testing.T) {
		boom := errors.New("smtp is down")
		sent, err := outbox.Dispatch(ctx, 10, func(context.Context, ports.Message) error { return boom })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sent != 0 {
			t.Fatalf("sent = %d, want 0", sent)
		}

		var attempts int
		var lastError string
		if err := pool.QueryRow(ctx,
			`SELECT attempts, last_error FROM outbox WHERE published_at IS NULL`).Scan(&attempts, &lastError); err != nil {
			t.Fatalf("read outbox: %v", err)
		}
		if attempts != 1 || lastError != boom.Error() {
			t.Fatalf("attempts = %d, last_error = %q", attempts, lastError)
		}
	})

	t.Run("a successful send marks it published", func(t *testing.T) {
		var seen []ports.Message
		sent, err := outbox.Dispatch(ctx, 10, func(_ context.Context, msg ports.Message) error {
			seen = append(seen, msg)
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sent != 1 || len(seen) != 1 {
			t.Fatalf("sent = %d, seen = %d", sent, len(seen))
		}
		if seen[0].Type != app.EventReservationConfirmed {
			t.Errorf("type = %q, want %q", seen[0].Type, app.EventReservationConfirmed)
		}

		sent, err = outbox.Dispatch(ctx, 10, func(context.Context, ports.Message) error {
			t.Error("published message was dispatched again")
			return nil
		})
		if err != nil || sent != 0 {
			t.Fatalf("second drain: sent = %d, err = %v", sent, err)
		}
	})

	t.Run("cancelling enqueues its own event", func(t *testing.T) {
		status, raw := do(t, http.MethodPost, srv.URL+"/v1/reservations/"+reference+"/cancel", nil)
		if status != http.StatusOK {
			t.Fatalf("cancel = %d %s", status, raw)
		}

		var seen []ports.Message
		if _, err := outbox.Dispatch(ctx, 10, func(_ context.Context, msg ports.Message) error {
			seen = append(seen, msg)
			return nil
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(seen) != 1 || seen[0].Type != app.EventReservationCancelled {
			t.Fatalf("messages = %+v", seen)
		}
	})
}
