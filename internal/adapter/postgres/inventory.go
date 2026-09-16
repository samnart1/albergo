package postgres

import (
	"context"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/samnart1/albergo/internal/domain/inventory"
	"github.com/samnart1/albergo/internal/domain/shared"
)

type InventoryRepository struct{ base }

func NewInventoryRepository(pool *pgxpool.Pool) *InventoryRepository {
	return &InventoryRepository{base{pool: pool}}
}

func (r *InventoryRepository) Allotments(ctx context.Context, roomTypeID uuid.UUID, from, to shared.Date) ([]inventory.Allotment, error) {
	const q = `
SELECT stay_date, allotment, booked
FROM inventory
WHERE room_type_id = $1 AND stay_date >= $2 AND stay_date < $3
ORDER BY stay_date`

	rows, err := r.db(ctx).Query(ctx, q, roomTypeID, from.Time(), to.Time())
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var allotments []inventory.Allotment
	for rows.Next() {
		var (
			stayDate         time.Time
			allotted, booked int16
		)
		if err := rows.Scan(&stayDate, &allotted, &booked); err != nil {
			return nil, mapError(err)
		}
		allotments = append(allotments, inventory.Allotment{
			Date:      shared.DateOf(stayDate),
			Allotment: int(allotted),
			Booked:    int(booked),
		})
	}
	return allotments, mapError(rows.Err())
}

func (r *InventoryRepository) UpsertAllotment(ctx context.Context, roomTypeID uuid.UUID, from, to shared.Date, allotment int) error {
	const q = `
INSERT INTO inventory (room_type_id, stay_date, allotment)
SELECT $1, d::date, $4
FROM generate_series($2::date, $3::date, interval '1 day') AS d
ON CONFLICT (room_type_id, stay_date) DO UPDATE SET allotment = EXCLUDED.allotment`

	_, err := r.db(ctx).Exec(ctx, q, roomTypeID, from.Time(), to.AddDays(-1).Time(), int16(allotment))
	return mapError(err)
}

const reserveSQL = `
WITH locked AS (
    SELECT stay_date
    FROM inventory
    WHERE room_type_id = $1 AND stay_date >= $2 AND stay_date < $3
    ORDER BY stay_date
    FOR UPDATE
)
UPDATE inventory i
SET booked = i.booked + $4
FROM locked l
WHERE i.room_type_id = $1 AND i.stay_date = l.stay_date`

func (r *InventoryRepository) Reserve(ctx context.Context, roomTypeID uuid.UUID, stay shared.Stay, units int) error {
	return r.move(ctx, roomTypeID, stay, units)
}

func (r *InventoryRepository) Release(ctx context.Context, roomTypeID uuid.UUID, stay shared.Stay, units int) error {
	return r.move(ctx, roomTypeID, stay, -units)
}

func (r *InventoryRepository) move(ctx context.Context, roomTypeID uuid.UUID, stay shared.Stay, delta int) error {
	tag, err := r.db(ctx).Exec(ctx, reserveSQL,
		roomTypeID, stay.CheckIn().Time(), stay.CheckOut().Time(), int16(delta))
	if err != nil {
		return mapError(err)
	}

	// TODO:
	if int(tag.RowsAffected()) != stay.Nights() {
		return shared.Unprocessable("no_inventory", "inventory is not published for every night of %s", stay)
	}
	return nil
}
