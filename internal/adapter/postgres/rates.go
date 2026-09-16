package postgres

import (
	"context"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/samnart1/albergo/internal/domain/pricing"
	"github.com/samnart1/albergo/internal/domain/shared"
)

type RateRepository struct{ base }

func NewRateRepository(pool *pgxpool.Pool) *RateRepository {
	return &RateRepository{base{pool: pool}}
}

func (r *RateRepository) Rates(ctx context.Context, ratePlanID uuid.UUID, from, to shared.Date) ([]pricing.RateDay, error) {
	const q = `
SELECT rc.stay_date, rc.price_cents, rc.min_stay, rc.closed_to_arrival, rc.closed_to_departure, p.currency
FROM rate_calendar rc
JOIN rate_plans rp ON rp.id = rc.rate_plan_id
JOIN properties p ON p.id = rp.property_id
WHERE rc.rate_plan_id = $1 AND rc.stay_date >= $2 AND rc.stay_date <= $3
ORDER BY rc.stay_date`

	rows, err := r.db(ctx).Query(ctx, q, ratePlanID, from.Time(), to.Time())
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var days []pricing.RateDay
	for rows.Next() {
		var (
			stayDate time.Time
			cents    int64
			minStay  int16
			cta, ctd bool
			currency shared.Currency
		)
		if err := rows.Scan(&stayDate, &cents, &minStay, &cta, &ctd, &currency); err != nil {
			return nil, mapError(err)
		}

		price, err := shared.NewMoney(cents, currency)
		if err != nil {
			return nil, shared.Internal("corrupt_rate", err)
		}

		days = append(days, pricing.RateDay{
			Date:              shared.DateOf(stayDate),
			Price:             price,
			MinStay:           int(minStay),
			ClosedToArrival:   cta,
			ClosedToDeparture: ctd,
		})
	}
	return days, mapError(rows.Err())
}

func (r *RateRepository) UpsertRates(ctx context.Context, ratePlanID uuid.UUID, days []pricing.RateDay) error {
	if len(days) == 0 {
		return nil
	}

	dates := make([]time.Time, len(days))
	cents := make([]int64, len(days))
	minStay := make([]int16, len(days))
	cta := make([]bool, len(days))
	ctd := make([]bool, len(days))

	for i, d := range days {
		dates[i] = d.Date.Time()
		cents[i] = d.Price.Cents()
		minStay[i] = int16(d.MinStay)
		cta[i] = d.ClosedToArrival
		ctd[i] = d.ClosedToDeparture
	}

	// TODO: fix bulk rate updates (works but slow atm)!
	const q = `
INSERT INTO rate_calendar (rate_plan_id, stay_date, price_cents, min_stay, closed_to_arrival, closed_to_departure)
SELECT $1, * FROM unnest($2::date[], $3::bigint[], $4::smallint[], $5::boolean[], $6::boolean[])
ON CONFLICT (rate_plan_id, stay_date) DO UPDATE SET
    price_cents         = EXCLUDED.price_cents,
    min_stay            = EXCLUDED.min_stay,
    closed_to_arrival   = EXCLUDED.closed_to_arrival,
    closed_to_departure = EXCLUDED.closed_to_departure`

	_, err := r.db(ctx).Exec(ctx, q, ratePlanID, dates, cents, minStay, cta, ctd)
	return mapError(err)
}
