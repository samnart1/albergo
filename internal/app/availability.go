package app

import (
	"context"
	"uuid"

	"github.com/samnart1/albergo/internal/app/ports"
	"github.com/samnart1/albergo/internal/domain/catalog"
	"github.com/samnart1/albergo/internal/domain/pricing"
	"github.com/samnart1/albergo/internal/domain/shared"
)

type AvailabilityService struct {
	catalog   ports.CatalogRepository
	rates     ports.RateRepository
	inventory ports.InventoryRepository
}

type SearchInput struct {
	PropertyID uuid.UUID
	Stay       shared.Stay
	Guests     int
}

type Offer struct {
	RoomType  catalog.RoomType
	RatePlan  catalog.RatePlan
	Quote     pricing.Quote
	UnitsLeft int
}

func (s *AvailabilityService) Search(ctx context.Context, in SearchInput) ([]Offer, error) {
	if in.Guests < 1 {
		return nil, shared.Invalid("invalid_guest_count", "guest count must be at least 1")
	}

	roomTypes, err := s.catalog.RoomTypes(ctx, in.PropertyID)
	if err != nil {
		return nil, err
	}
	if len(roomTypes) == 0 {
		return nil, shared.NotFound("property_not_found", "no property with id %s", in.PropertyID)
	}

	ratePlans, err := s.catalog.RatePlans(ctx, in.PropertyID)
	if err != nil {
		return nil, err
	}

	byRoomType := make(map[uuid.UUID]catalog.RoomType, len(roomTypes))
	for _, rt := range roomTypes {
		byRoomType[rt.ID] = rt
	}

	offers := make([]Offer, 0, len(ratePlans))
	for _, rp := range ratePlans {
		roomType, ok := byRoomType[rp.RoomTypeID]
		if !ok || roomType.MaxOccupancy < in.Guests {
			continue
		}

		rates, err := s.rates.Rates(ctx, rp.ID, in.Stay.CheckIn(), in.Stay.CheckOut())
		if err != nil {
			return nil, err
		}

		quote, err := pricing.Calculate(in.Stay, rates)
		if err != nil {
			if shared.KindOf(err) == shared.KindInternal {
				return nil, err
			}
			continue
		}

		unitsLeft, err := s.unitsLeft(ctx, roomType.ID, in.Stay)
		if err != nil {
			return nil, err
		}
		if unitsLeft < 1 {
			continue
		}

		offers = append(offers, Offer{RoomType: roomType, RatePlan: rp, Quote: quote, UnitsLeft: unitsLeft})
	}
	return offers, nil
}

func (s *AvailabilityService) unitsLeft(ctx context.Context, roomTypeID uuid.UUID, stay shared.Stay) (int, error) {
	allotments, err := s.inventory.Allotments(ctx, roomTypeID, stay.CheckIn(), stay.CheckOut())
	if err != nil {
		return 0, err
	}
	if len(allotments) != stay.Nights() {
		return 0, nil
	}

	left := allotments[0].Available()
	for _, a := range allotments[1:] {
		if a.Available() < left {
			left = a.Available()
		}
	}
	return left, nil
}
