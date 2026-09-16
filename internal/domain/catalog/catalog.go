package catalog

import (
	"uuid"

	"github.com/samnart1/albergo/internal/domain/shared"
)

type RoomType struct {
	ID           uuid.UUID
	PropertyID   uuid.UUID
	Code         string
	Name         string
	MaxOccupancy int
	Currency     shared.Currency
}

type RatePlan struct {
	ID                   uuid.UUID
	PropertyID           uuid.UUID
	RoomTypeID           uuid.UUID
	Code                 string
	RefundableUntilHours int
}
