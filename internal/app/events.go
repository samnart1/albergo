package app

import (
	"github.com/samnart1/albergo/internal/domain/booking"
	"github.com/samnart1/albergo/internal/domain/shared"
)

const (
	EventReservationConfirmed = "reservation.confirmed"
	EventReservationCancelled = "reservation.cancelled"
)

type reservationEvent struct {
	Reference   string      `json:"reference"`
	Status      string      `json:"status"`
	PropertyID  string      `json:"property_id"`
	GuestEmail  string      `json:"guest_email"`
	GuestName   string      `json:"guest_name"`
	CheckIn     shared.Date `json:"check_in"`
	CheckOut    shared.Date `json:"check_out"`
	Guests      int         `json:"guests"`
	TotalCents  int64       `json:"total_cents"`
	Currency    string      `json:"currency"`
	RefundCents int64       `json:"refund_cents,omitempty"`
	Refundable  bool        `json:"refundable,omitempty"`
}

func newReservationEvent(res *booking.Reservation, guest booking.Guest) reservationEvent {
	return reservationEvent{
		Reference:  res.Reference,
		Status:     string(res.Status),
		PropertyID: res.PropertyID.String(),
		GuestEmail: guest.Email,
		GuestName:  guest.FullName,
		CheckIn:    res.Stay.CheckIn(),
		CheckOut:   res.Stay.CheckOut(),
		Guests:     res.GuestCount,
		TotalCents: res.Total.Cents(),
		Currency:   string(res.Total.Currency()),
	}
}
