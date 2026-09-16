package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/samnart1/albergo/internal/app/ports"
	"github.com/samnart1/albergo/internal/domain/shared"
)

// max retries
const maxAttempts = 5

type OutboxRepository struct{ base }

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{base{pool: pool}}
}

func (r *OutboxRepository) Publish(ctx context.Context, event ports.Event) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return shared.Internal("event_encode_failed", err)
	}

	const q = `INSERT INTO outbox (aggregate_id, type, payload) VALUES ($1, $2, $3)`

	_, err = r.db(ctx).Exec(ctx, q, event.AggregateID, event.Type, payload)
	return mapError(err)
}

func (r *OutboxRepository) Dispatch(ctx context.Context, batch int, send func(ctx context.Context, msg ports.Message) error) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, shared.Internal("begin_failed", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const claim = `
WITH candidates AS (
    SELECT id FROM outbox
    WHERE published_at IS NULL AND attempts < $1
    ORDER BY id
    LIMIT $2
    FOR UPDATE SKIP LOCKED
)
SELECT o.id, o.aggregate_id, o.type, o.payload, o.attempts
FROM outbox o JOIN candidates c ON c.id = o.id
ORDER BY o.id`

	rows, err := tx.Query(ctx, claim, maxAttempts, batch)
	if err != nil {
		return 0, mapError(err)
	}

	var messages []ports.Message
	for rows.Next() {
		var msg ports.Message
		if err := rows.Scan(&msg.ID, &msg.AggregateID, &msg.Type, &msg.Payload, &msg.Attempts); err != nil {
			rows.Close()
			return 0, mapError(err)
		}
		messages = append(messages, msg)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, mapError(err)
	}

	const markSent = `UPDATE outbox SET published_at = now() WHERE id = $1`
	const markFailed = `UPDATE outbox SET attempts = attempts + 1, last_error = $2 WHERE id = $1`

	sent := 0
	for _, msg := range messages {
		if sendErr := send(ctx, msg); sendErr != nil {
			if _, err := tx.Exec(ctx, markFailed, msg.ID, sendErr.Error()); err != nil {
				return sent, mapError(err)
			}
			continue
		}
		if _, err := tx.Exec(ctx, markSent, msg.ID); err != nil {
			return sent, mapError(err)
		}
		sent++
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, shared.Internal("commit_failed", err)
	}
	return sent, nil
}
