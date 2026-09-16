package app

import (
	"github.com/samnart1/albergo/internal/app/ports"
	"github.com/samnart1/albergo/internal/domain/shared"
)

type Services struct {
	Availability *AvailabilityService
	Reservations *ReservationService
	Staff        *StaffService
}

type Deps struct {
	Tx           ports.TxManager
	Catalog      ports.CatalogRepository
	Rates        ports.RateRepository
	Inventory    ports.InventoryRepository
	Guests       ports.GuestRepository
	Reservations ports.ReservationRepository
	Clock        shared.Clock
	Idempotency  ports.IdempotencyRepository
	Outbox       ports.OutboxRepository
}

// single composition point
func New(d Deps) *Services {
	for _, dep := range []struct {
		name string
		val  any
	}{
		{"Tx", d.Tx}, {"Catalog", d.Catalog}, {"Rates", d.Rates}, {"Inventory", d.Inventory},
		{"Guests", d.Guests}, {"Reservations", d.Reservations},
		{"Idempotency", d.Idempotency}, {"Outbox", d.Outbox}, {"Clock", d.Clock},
	} {
		if dep.val == nil {
			panic("app: Deps." + dep.name + " is required")
		}
	}

	return &Services{
		Availability: &AvailabilityService{catalog: d.Catalog, rates: d.Rates, inventory: d.Inventory},
		Reservations: &ReservationService{
			tx: d.Tx, catalog: d.Catalog, rates: d.Rates, inventory: d.Inventory,
			guests: d.Guests, reservations: d.Reservations,
			idempotency: d.Idempotency, outbox: d.Outbox, clock: d.Clock,
		},
		Staff: &StaffService{catalog: d.Catalog, rates: d.Rates, inventory: d.Inventory, reservations: d.Reservations},
	}
}
