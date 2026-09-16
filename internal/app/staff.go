package app

import (
	"context"
	"uuid"

	"github.com/samnart1/albergo/internal/app/ports"
	"github.com/samnart1/albergo/internal/domain/booking"
	"github.com/samnart1/albergo/internal/domain/pricing"
	"github.com/samnart1/albergo/internal/domain/shared"
)

type StaffService struct {
	catalog      ports.CatalogRepository
	rates        ports.RateRepository
	inventory    ports.InventoryRepository
	reservations ports.ReservationRepository
}

type RateInput struct {
	Date              shared.Date
	PriceCents        int64
	MinStay           int
	ClosedToArrival   bool
	ClosedToDeparture bool
}

func (s *StaffService) SetInventory(ctx context.Context, roomTypeID uuid.UUID, from, to shared.Date, allotment int) error {
	if allotment < 0 {
		return shared.Invalid("invalid_allotment", "allotment must not be negative")
	}
	if !to.After(from) {
		return shared.Invalid("invalid_range", "to %s must be after from %s", to, from)
	}
	if _, err := s.catalog.RoomType(ctx, roomTypeID); err != nil {
		return err
	}
	return s.inventory.UpsertAllotment(ctx, roomTypeID, from, to, allotment)
}

func (s *StaffService) SetRates(ctx context.Context, ratePlanID uuid.UUID, inputs []RateInput) error {
	if len(inputs) == 0 {
		return shared.Invalid("no_rates", "at least one rate is required")
	}

	ratePlan, err := s.catalog.RatePlan(ctx, ratePlanID)
	if err != nil {
		return err
	}
	// currency is property att. a typo requst can't introduce a new currency
	roomType, err := s.catalog.RoomType(ctx, ratePlan.RoomTypeID)
	if err != nil {
		return err
	}

	days := make([]pricing.RateDay, 0, len(inputs))
	for _, in := range inputs {
		if in.Date.IsZero() {
			return shared.Invalid("invalid_rate", "every rate needs a date")
		}
		if in.MinStay < 1 {
			return shared.Invalid("invalid_rate", "minimum stay on %s must be at least 1", in.Date)
		}

		price, err := shared.NewMoney(in.PriceCents, roomType.Currency)
		if err != nil {
			return err
		}

		days = append(days, pricing.RateDay{
			Date:              in.Date,
			Price:             price,
			MinStay:           in.MinStay,
			ClosedToArrival:   in.ClosedToArrival,
			ClosedToDeparture: in.ClosedToDeparture,
		})
	}
	return s.rates.UpsertRates(ctx, ratePlanID, days)
}

func (s *StaffService) Reservations(ctx context.Context, propertyID uuid.UUID, from, to shared.Date) ([]*booking.Reservation, error) {
	if !to.After(from) {
		return nil, shared.Invalid("invalid_range", "to %s must be after from %s", to, from)
	}
	return s.reservations.ListByProperty(ctx, propertyID, from, to)
}
