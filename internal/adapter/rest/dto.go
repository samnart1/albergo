package rest

import (
	"time"
	"uuid"

	"github.com/samnart1/albergo/internal/app"
	"github.com/samnart1/albergo/internal/domain/booking"
	"github.com/samnart1/albergo/internal/domain/pricing"
	"github.com/samnart1/albergo/internal/domain/shared"
)

type moneyDTO struct {
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
}

func toMoney(m shared.Money) moneyDTO {
	return moneyDTO{AmountCents: m.Cents(), Currency: string(m.Currency())}
}

type nightDTO struct {
	Date  shared.Date `json:"date"`
	Price moneyDTO    `json:"price"`
}

func toNights(nights []pricing.Night) []nightDTO {
	out := make([]nightDTO, 0, len(nights))
	for _, n := range nights {
		out = append(out, nightDTO{Date: n.Date, Price: toMoney(n.Price)})
	}
	return out
}

type offerDTO struct {
	RoomTypeID   uuid.UUID  `json:"room_type_id"`
	RoomTypeName string     `json:"room_type_name"`
	MaxOccupancy int        `json:"max_occupancy"`
	RatePlanID   uuid.UUID  `json:"rate_plan_id"`
	RatePlanCode string     `json:"rate_plan_code"`
	UnitsLeft    int        `json:"units_left"`
	Total        moneyDTO   `json:"total"`
	Nights       []nightDTO `json:"nights"`
}

type availabilityResponse struct {
	CheckIn  shared.Date `json:"check_in"`
	CheckOut shared.Date `json:"check_out"`
	Nights   int         `json:"nights"`
	Offers   []offerDTO  `json:"offers"`
}

func toOffers(offers []app.Offer) []offerDTO {
	out := make([]offerDTO, 0, len(offers))
	for _, o := range offers {
		out = append(out, offerDTO{
			RoomTypeID:   o.RoomType.ID,
			RoomTypeName: o.RoomType.Name,
			MaxOccupancy: o.RoomType.MaxOccupancy,
			RatePlanID:   o.RatePlan.ID,
			RatePlanCode: o.RatePlan.Code,
			UnitsLeft:    o.UnitsLeft,
			Total:        toMoney(o.Quote.Total),
			Nights:       toNights(o.Quote.Nights),
		})
	}
	return out
}

type guestDTO struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
}

type createReservationRequest struct {
	PropertyID uuid.UUID   `json:"property_id"`
	RoomTypeID uuid.UUID   `json:"room_type_id"`
	RatePlanID uuid.UUID   `json:"rate_plan_id"`
	CheckIn    shared.Date `json:"check_in"`
	CheckOut   shared.Date `json:"check_out"`
	Guests     int         `json:"guests"`
	Guest      guestDTO    `json:"guest"`
}

type reservationDTO struct {
	Reference   string      `json:"reference"`
	Status      string      `json:"status"`
	PropertyID  uuid.UUID   `json:"property_id"`
	RoomTypeID  uuid.UUID   `json:"room_type_id"`
	RatePlanID  uuid.UUID   `json:"rate_plan_id"`
	CheckIn     shared.Date `json:"check_in"`
	CheckOut    shared.Date `json:"check_out"`
	Guests      int         `json:"guests"`
	Total       moneyDTO    `json:"total"`
	Nights      []nightDTO  `json:"nights,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	CancelledAt *time.Time  `json:"cancelled_at,omitempty"`
}

func toReservation(res *booking.Reservation) reservationDTO {
	return reservationDTO{
		Reference:   res.Reference,
		Status:      string(res.Status),
		PropertyID:  res.PropertyID,
		RoomTypeID:  res.RoomTypeID,
		RatePlanID:  res.RatePlanID,
		CheckIn:     res.Stay.CheckIn(),
		CheckOut:    res.Stay.CheckOut(),
		Guests:      res.GuestCount,
		Total:       toMoney(res.Total),
		Nights:      toNights(res.Nights),
		CreatedAt:   res.CreatedAt,
		CancelledAt: res.CancelledAt,
	}
}

type cancelResponse struct {
	Reservation reservationDTO `json:"reservation"`
	Refund      moneyDTO       `json:"refund"`
	Refundable  bool           `json:"refundable"`
}

type reservationListResponse struct {
	Reservations []reservationDTO `json:"reservations"`
}

type setInventoryRequest struct {
	From      shared.Date `json:"from"`
	To        shared.Date `json:"to"`
	Allotment int         `json:"allotment"`
}

type rateDTO struct {
	Date              shared.Date `json:"date"`
	PriceCents        int64       `json:"price_cents"`
	MinStay           int         `json:"min_stay"`
	ClosedToArrival   bool        `json:"closed_to_arrival"`
	ClosedToDeparture bool        `json:"closed_to_departure"`
}

type setRatesRequest struct {
	Rates []rateDTO `json:"rates"`
}
