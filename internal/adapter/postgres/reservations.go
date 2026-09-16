package postgres

import (
	"context"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/samnart1/albergo/internal/domain/booking"
	"github.com/samnart1/albergo/internal/domain/pricing"
	"github.com/samnart1/albergo/internal/domain/shared"
)

type ReservationRepository struct{ base }

func NewReservationRepository(pool *pgxpool.Pool) *ReservationRepository {
	return &ReservationRepository{base{pool: pool}}
}

func (r *ReservationRepository) Create(ctx context.Context, res *booking.Reservation) error {
	const insertReservation = `
INSERT INTO reservations (
    id, reference, property_id, room_type_id, rate_plan_id, guest_id,
    check_in, check_out, guest_count, status, total_cents, currency, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::reservation_status, $11, $12, $13)`

	_, err := r.db(ctx).Exec(ctx, insertReservation,
		res.ID, res.Reference, res.PropertyID, res.RoomTypeID, res.RatePlanID, res.GuestID,
		res.Stay.CheckIn().Time(), res.Stay.CheckOut().Time(), int16(res.GuestCount),
		string(res.Status), res.Total.Cents(), string(res.Total.Currency()), res.CreatedAt,
	)
	if err != nil {
		return mapError(err)
	}

	dates := make([]time.Time, len(res.Nights))
	cents := make([]int64, len(res.Nights))
	for i, n := range res.Nights {
		dates[i] = n.Date.Time()
		cents[i] = n.Price.Cents()
	}

	// don't overwrite current price with future price
	const insertNights = `
INSERT INTO reservation_nights (reservation_id, stay_date, price_cents)
SELECT $1, * FROM unnest($2::date[], $3::bigint[])`

	_, err = r.db(ctx).Exec(ctx, insertNights, res.ID, dates, cents)
	return mapError(err)
}

const reservationColumns = `
    id, reference, property_id, room_type_id, rate_plan_id, guest_id,
    check_in, check_out, guest_count, status::text, total_cents, currency, created_at, cancelled_at`

func (r *ReservationRepository) ByID(ctx context.Context, id uuid.UUID) (*booking.Reservation, error) {
	row := r.db(ctx).QueryRow(ctx, `SELECT`+reservationColumns+` FROM reservations WHERE id = $1`, id)
	return r.scan(ctx, row)
}

func (r *ReservationRepository) ByReference(ctx context.Context, reference string) (*booking.Reservation, error) {
	row := r.db(ctx).QueryRow(ctx, `SELECT`+reservationColumns+` FROM reservations WHERE reference = $1`, reference)
	return r.scan(ctx, row)
}

func (r *ReservationRepository) scan(ctx context.Context, row pgx.Row) (*booking.Reservation, error) {
	res, err := scanReservation(row)
	if err != nil {
		return nil, err
	}

	nights, err := r.nights(ctx, res.ID, res.Total.Currency())
	if err != nil {
		return nil, err
	}
	res.Nights = nights

	return res, nil
}

func scanReservation(row pgx.Row) (*booking.Reservation, error) {
	var (
		res               booking.Reservation
		checkIn, checkOut time.Time
		guestCount        int16
		status            string
		totalCents        int64
		currency          shared.Currency
		cancelledAt       *time.Time
	)

	err := row.Scan(
		&res.ID, &res.Reference, &res.PropertyID, &res.RoomTypeID, &res.RatePlanID, &res.GuestID,
		&checkIn, &checkOut, &guestCount, &status, &totalCents, &currency, &res.CreatedAt, &cancelledAt,
	)
	if err != nil {
		return nil, mapError(err)
	}

	stay, err := shared.NewStay(shared.DateOf(checkIn), shared.DateOf(checkOut))
	if err != nil {
		return nil, shared.Internal("corrupt_reservation", err)
	}
	total, err := shared.NewMoney(totalCents, currency)
	if err != nil {
		return nil, shared.Internal("corrupt_reservation", err)
	}

	res.Stay = stay
	res.GuestCount = int(guestCount)
	res.Status = booking.Status(status)
	res.Total = total
	res.CancelledAt = cancelledAt

	return &res, nil
}

func (r *ReservationRepository) nights(ctx context.Context, id uuid.UUID, currency shared.Currency) ([]pricing.Night, error) {
	const q = `SELECT stay_date, price_cents FROM reservation_nights WHERE reservation_id = $1 ORDER BY stay_date`

	rows, err := r.db(ctx).Query(ctx, q, id)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var nights []pricing.Night
	for rows.Next() {
		var (
			stayDate time.Time
			cents    int64
		)
		if err := rows.Scan(&stayDate, &cents); err != nil {
			return nil, mapError(err)
		}
		price, err := shared.NewMoney(cents, currency)
		if err != nil {
			return nil, shared.Internal("corrupt_reservation", err)
		}
		nights = append(nights, pricing.Night{Date: shared.DateOf(stayDate), Price: price})
	}
	return nights, mapError(rows.Err())
}

func (r *ReservationRepository) Cancel(ctx context.Context, res *booking.Reservation) error {
	const q = `
UPDATE reservations
SET status = $2::reservation_status, cancelled_at = $3
WHERE id = $1 AND status = 'confirmed'`

	tag, err := r.db(ctx).Exec(ctx, q, res.ID, string(res.Status), res.CancelledAt)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return shared.Conflict("already_cancelled", "reservation %s is already cancelled", res.Reference)
	}
	return nil
}

func (r *ReservationRepository) ListByProperty(ctx context.Context, propertyID uuid.UUID, from, to shared.Date) ([]*booking.Reservation, error) {
	const q = `SELECT` + reservationColumns + `
FROM reservations
WHERE property_id = $1 AND check_in >= $2 AND check_in < $3
ORDER BY check_in, reference`

	rows, err := r.db(ctx).Query(ctx, q, propertyID, from.Time(), to.Time())
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var out []*booking.Reservation
	for rows.Next() {
		res, err := scanReservation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, mapError(rows.Err())
}
