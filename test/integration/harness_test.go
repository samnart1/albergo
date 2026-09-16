//go:build integration

package integration

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/samnart1/albergo/internal/platform/db"
	"github.com/samnart1/albergo/migrations"
	"github.com/samnart1/albergo/test/contract"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:18-alpine",
		tcpostgres.WithDatabase("albergo"),
		tcpostgres.WithUsername("albergo"),
		tcpostgres.WithPassword("albergo"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatalf("start postgres: %v", err)
	}

	code := func() int {
		defer func() {
			if err := testcontainers.TerminateContainer(container); err != nil {
				log.Printf("terminate: %v", err)
			}
		}()

		dsn, err := container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			log.Fatalf("connection string: %v", err)
		}

		pool, err = db.Open(ctx, dsn)
		if err != nil {
			log.Fatalf("open pool: %v", err)
		}
		defer pool.Close()

		if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
			log.Fatalf("migrate: %v", err)
		}

		return m.Run()
	}()

	os.Exit(code)
}

func truncate(t *testing.T) {
	t.Helper()

	const q = `
TRUNCATE properties, room_types, rate_plans, rate_calendar, inventory,
         guests, reservations, reservation_nights, outbox, idempotency_keys
RESTART IDENTITY CASCADE`

	if _, err := pool.Exec(context.Background(), q); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func seed(t *testing.T) contract.Fixture {
	t.Helper()
	ctx := context.Background()

	fx := contract.Fixture{
		PropertyID:   uuid.New(),
		RoomTypeID:   uuid.New(),
		RatePlanID:   uuid.New(),
		MaxOccupancy: 2,
	}

	const q = `
WITH p AS (
    INSERT INTO properties (id, name, city) VALUES ($1, 'Hotel Test', 'Bozen') RETURNING id
), rt AS (
    INSERT INTO room_types (id, property_id, code, name, max_occupancy)
    VALUES ($2, $1, 'DBL', 'Doppelzimmer', $4) RETURNING id
)
INSERT INTO rate_plans (id, property_id, room_type_id, code, name, refundable_until_hours)
VALUES ($3, $1, $2, 'FLEX', 'Flexible', 48)`

	if _, err := pool.Exec(ctx, q, fx.PropertyID, fx.RoomTypeID, fx.RatePlanID, int16(fx.MaxOccupancy)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return fx
}

func ping(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

var _ = fmt.Sprintf
