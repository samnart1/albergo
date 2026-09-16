package postgres

import (
	"context"
	"errors"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/samnart1/albergo/internal/app/ports"
	"github.com/samnart1/albergo/internal/domain/shared"
)

type IdempotencyRepository struct{ base }

func NewIdempotencyRepository(pool *pgxpool.Pool) *IdempotencyRepository {
	return &IdempotencyRepository{base{pool: pool}}
}

// takes the key and reports who has it
func (r *IdempotencyRepository) Claim(ctx context.Context, key, requestHash string) (ports.Claim, error) {
	const insert = `
INSERT INTO idempotency_keys (key, request_hash) VALUES ($1, $2)
ON CONFLICT (key) DO NOTHING`

	tag, err := r.db(ctx).Exec(ctx, insert, key, requestHash)
	if err != nil {
		return ports.Claim{}, mapError(err)
	}
	if tag.RowsAffected() == 1 {
		return ports.Claim{Fresh: true, RequestHash: requestHash}, nil
	}

	const read = `SELECT request_hash, reservation_id FROM idempotency_keys WHERE key = $1`

	var claim ports.Claim
	err = r.db(ctx).QueryRow(ctx, read, key).Scan(&claim.RequestHash, &claim.ReservationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ports.Claim{Fresh: true, RequestHash: requestHash}, nil
		}
		return ports.Claim{}, mapError(err)
	}
	return claim, nil
}

func (r *IdempotencyRepository) Record(ctx context.Context, key string, reservationID uuid.UUID) error {
	const q = `UPDATE idempotency_keys SET reservation_id = $2 WHERE key = $1`

	tag, err := r.db(ctx).Exec(ctx, q, key, reservationID)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return shared.Internal("idempotency_key_lost", errors.New("key vanished before it could be recorded"))
	}
	return nil
}
