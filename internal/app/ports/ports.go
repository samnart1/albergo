package ports

import (
	"context"
	"uuid"

	"github.com/samnart1/albergo/internal/domain/booking"
	"github.com/samnart1/albergo/internal/domain/catalog"
	"github.com/samnart1/albergo/internal/domain/inventory"
	"github.com/samnart1/albergo/internal/domain/pricing"
	"github.com/samnart1/albergo/internal/domain/shared"
)

type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type CatalogRepository interface {
	RoomType(ctx context.Context, id uuid.UUID) (catalog.RoomType, error)
	RoomTypes(ctx context.Context, propertyID uuid.UUID) ([]catalog.RoomType, error)
	RatePlan(ctx context.Context, id uuid.UUID) (catalog.RatePlan, error)
	RatePlans(ctx context.Context, propertyID uuid.UUID) ([]catalog.RatePlan, error)
}

type RateRepository interface {
	Rates(ctx context.Context, ratePlanID uuid.UUID, from, to shared.Date) ([]pricing.RateDay, error)
	UpsertRates(ctx context.Context, ratePlanID uuid.UUID, days []pricing.RateDay) error
}

type InventoryRepository interface {
	Allotments(ctx context.Context, roomTypeID uuid.UUID, from, to shared.Date) ([]inventory.Allotment, error)
	UpsertAllotment(ctx context.Context, roomTypeID uuid.UUID, from, to shared.Date, allotment int) error
	Reserve(ctx context.Context, roomTypeID uuid.UUID, stay shared.Stay, units int) error
	Release(ctx context.Context, roomTypeID uuid.UUID, stay shared.Stay, units int) error
}

type GuestRepository interface {
	Upsert(ctx context.Context, guest booking.Guest) (booking.Guest, error)
	ByID(ctx context.Context, id uuid.UUID) (booking.Guest, error)
}

type ReservationRepository interface {
	Create(ctx context.Context, res *booking.Reservation) error
	ByID(ctx context.Context, id uuid.UUID) (*booking.Reservation, error)
	ByReference(ctx context.Context, reference string) (*booking.Reservation, error)
	Cancel(ctx context.Context, res *booking.Reservation) error
	ListByProperty(ctx context.Context, propertyID uuid.UUID, from, to shared.Date) ([]*booking.Reservation, error)
}
