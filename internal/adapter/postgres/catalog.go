package postgres

import (
	"context"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/samnart1/albergo/internal/domain/catalog"
)

type CatalogRepository struct{ base }

func NewCatalogRepository(pool *pgxpool.Pool) *CatalogRepository {
	return &CatalogRepository{base{pool: pool}}
}

func (r *CatalogRepository) RoomType(ctx context.Context, id uuid.UUID) (catalog.RoomType, error) {
	const q = `
SELECT rt.id, rt.property_id, rt.code, rt.name, rt.max_occupancy, p.currency
FROM room_types rt
JOIN properties p ON p.id = rt.property_id
WHERE rt.id = $1`

	var rt catalog.RoomType
	err := r.db(ctx).QueryRow(ctx, q, id).Scan(
		&rt.ID, &rt.PropertyID, &rt.Code, &rt.Name, &rt.MaxOccupancy, &rt.Currency,
	)
	if err != nil {
		return catalog.RoomType{}, mapError(err)
	}
	return rt, nil
}

func (r *CatalogRepository) RatePlan(ctx context.Context, id uuid.UUID) (catalog.RatePlan, error) {
	const q = `
SELECT id, property_id, room_type_id, code, refundable_until_hours
FROM rate_plans
WHERE id = $1`

	var rp catalog.RatePlan
	err := r.db(ctx).QueryRow(ctx, q, id).Scan(
		&rp.ID, &rp.PropertyID, &rp.RoomTypeID, &rp.Code, &rp.RefundableUntilHours,
	)
	if err != nil {
		return catalog.RatePlan{}, mapError(err)
	}
	return rp, nil
}

func (r *CatalogRepository) RoomTypes(ctx context.Context, propertyID uuid.UUID) ([]catalog.RoomType, error) {
	const q = `
SELECT rt.id, rt.property_id, rt.code, rt.name, rt.max_occupancy, p.currency
FROM room_types rt
JOIN properties p ON p.id = rt.property_id
WHERE rt.property_id = $1
ORDER BY rt.code`

	rows, err := r.db(ctx).Query(ctx, q, propertyID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var out []catalog.RoomType
	for rows.Next() {
		var rt catalog.RoomType
		if err := rows.Scan(&rt.ID, &rt.PropertyID, &rt.Code, &rt.Name, &rt.MaxOccupancy, &rt.Currency); err != nil {
			return nil, mapError(err)
		}
		out = append(out, rt)
	}
	return out, mapError(rows.Err())
}

func (r *CatalogRepository) RatePlans(ctx context.Context, propertyID uuid.UUID) ([]catalog.RatePlan, error) {
	const q = `
SELECT id, property_id, room_type_id, code, refundable_until_hours
FROM rate_plans
WHERE property_id = $1
ORDER BY code`

	rows, err := r.db(ctx).Query(ctx, q, propertyID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var out []catalog.RatePlan
	for rows.Next() {
		var rp catalog.RatePlan
		if err := rows.Scan(&rp.ID, &rp.PropertyID, &rp.RoomTypeID, &rp.Code, &rp.RefundableUntilHours); err != nil {
			return nil, mapError(err)
		}
		out = append(out, rp)
	}
	return out, mapError(rows.Err())
}
