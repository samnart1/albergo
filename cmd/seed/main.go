package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
	"uuid"

	"github.com/samnart1/albergo/internal/adapter/postgres"
	"github.com/samnart1/albergo/internal/app"
	"github.com/samnart1/albergo/internal/domain/shared"
	"github.com/samnart1/albergo/internal/platform/db"
)

// seed, testing purposes
func main() {
	ctx := context.Background()

	pool, err := db.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	propertyID := uuid.New()
	const insert = `
WITH p AS (
    INSERT INTO properties (id, name, city) VALUES ($1, 'Hotel Laurin', 'Bozen') RETURNING id
), rt AS (
    INSERT INTO room_types (id, property_id, code, name, max_occupancy)
    VALUES ($2, $1, 'DBL', 'Doppelzimmer', 2), ($3, $1, 'SUITE', 'Suite', 4)
    RETURNING id
)
INSERT INTO rate_plans (id, property_id, room_type_id, code, name, refundable_until_hours)
VALUES ($4, $1, $2, 'FLEX-DBL', 'Flexible double', 48),
       ($5, $1, $3, 'FLEX-SUITE', 'Flexible suite', 48)`

	ids := struct{ dbl, suite, flexDBL, flexSuite uuid.UUID }{uuid.New(), uuid.New(), uuid.New(), uuid.New()}

	if _, err := pool.Exec(ctx, insert, propertyID, ids.dbl, ids.suite, ids.flexDBL, ids.flexSuite); err != nil {
		log.Fatal(err)
	}

	services := app.New(app.Deps{
		Tx:           postgres.NewTxManager(pool),
		Catalog:      postgres.NewCatalogRepository(pool),
		Rates:        postgres.NewRateRepository(pool),
		Inventory:    postgres.NewInventoryRepository(pool),
		Guests:       postgres.NewGuestRepository(pool),
		Reservations: postgres.NewReservationRepository(pool),
		Clock:        shared.SystemClock{},
		Idempotency:  postgres.NewIdempotencyRepository(pool),
		Outbox:       postgres.NewOutboxRepository(pool),
	})

	from := shared.DateOf(time.Now())
	to := from.AddDays(90)

	for _, room := range []struct {
		roomTypeID, ratePlanID uuid.UUID
		units                  int
		cents                  int64
	}{
		{ids.dbl, ids.flexDBL, 4, 14_500},
		{ids.suite, ids.flexSuite, 2, 32_000},
	} {
		if err := services.Staff.SetInventory(ctx, room.roomTypeID, from, to, room.units); err != nil {
			log.Fatal(err)
		}

		rates := make([]app.RateInput, 0, 90)
		for d := from; d.Before(to); d = d.AddDays(1) {
			rates = append(rates, app.RateInput{Date: d, PriceCents: room.cents, MinStay: 1})
		}
		if err := services.Staff.SetRates(ctx, room.ratePlanID, rates); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("property_id   %s\n", propertyID)
	fmt.Printf("room_type_id  %s (DBL)\n", ids.dbl)
	fmt.Printf("rate_plan_id  %s (FLEX-DBL)\n", ids.flexDBL)
	fmt.Printf("availability  /v1/properties/%s/availability?check_in=%s&check_out=%s&guests=2\n",
		propertyID, from.AddDays(1), from.AddDays(4))
}
