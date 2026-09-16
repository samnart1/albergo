package postgres

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/samnart1/albergo/internal/domain/booking"
)

type GuestRepository struct{ base }

func NewGuestRepository(pool *pgxpool.Pool) *GuestRepository {
	return &GuestRepository{base{pool: pool}}
}

func (r *GuestRepository) Upsert(ctx context.Context, guest booking.Guest) (booking.Guest, error) {
	const q = `
INSERT INTO guests (id, email, full_name, phone)
VALUES ($1, $2, $3, $4)
ON CONFLICT (lower(email)) DO UPDATE SET full_name = EXCLUDED.full_name, phone = EXCLUDED.phone
RETURNING id`

	err := r.db(ctx).QueryRow(ctx, q, guest.ID, guest.Email, guest.FullName, guest.Phone).Scan(&guest.ID)
	if err != nil {
		return booking.Guest{}, mapError(err)
	}
	return guest, nil
}

func (r *GuestRepository) ByID(ctx context.Context, id uuid.UUID) (booking.Guest, error) {
	const q = `SELECT id, email, full_name, coalesce(phone, '') FROM guests WHERE id = $1`

	var guest booking.Guest
	if err := r.db(ctx).QueryRow(ctx, q, id).Scan(&guest.ID, &guest.Email, &guest.FullName, &guest.Phone); err != nil {
		return booking.Guest{}, mapError(err)
	}
	return guest, nil
}
