//go:build integration

package integration

import (
	"testing"

	"github.com/samnart1/albergo/internal/adapter/postgres"
	"github.com/samnart1/albergo/test/contract"
)

func TestPostgresRepositories(t *testing.T) {
	contract.Run(t, func(t *testing.T) (contract.Repos, contract.Fixture) {
		truncate(t)

		return contract.Repos{
			Tx:           postgres.NewTxManager(pool),
			Catalog:      postgres.NewCatalogRepository(pool),
			Rates:        postgres.NewRateRepository(pool),
			Inventory:    postgres.NewInventoryRepository(pool),
			Guests:       postgres.NewGuestRepository(pool),
			Reservations: postgres.NewReservationRepository(pool),
		}, seed(t)
	})
}
