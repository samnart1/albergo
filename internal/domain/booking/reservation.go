package booking

import (
	"crypto/rand"
	"time"
	"uuid"

	"github.com/samnart1/albergo/internal/domain/pricing"
	"github.com/samnart1/albergo/internal/domain/shared"
)

type Status string

const (
	StatusConfirmed Status = "confirmed"
	StatusCancelled Status = "cancelled"
)

type Reservation struct {
	ID          uuid.UUID
	Reference   string
	PropertyID  uuid.UUID
	RoomTypeID  uuid.UUID
	RatePlanID  uuid.UUID
	GuestID     uuid.UUID
	Stay        shared.Stay
	GuestCount  int
	Status      Status
	Nights      []pricing.Night
	Total       shared.Money
	CreatedAt   time.Time
	CancelledAt *time.Time
}

type NewInput struct {
	PropertyID   uuid.UUID
	RoomTypeID   uuid.UUID
	RatePlanID   uuid.UUID
	Guest        Guest
	Quote        pricing.Quote
	GuestCount   int
	MaxOccupancy int
	Now          time.Time
}

func New(in NewInput) (*Reservation, error) {
	switch {
	case in.PropertyID == uuid.Nil(), in.RoomTypeID == uuid.Nil(),
		in.RatePlanID == uuid.Nil(), in.Guest.ID == uuid.Nil():
		return nil, shared.Invalid("missing_identifier", "property, room type, rate plan and guest are required")
	case in.GuestCount < 1:
		return nil, shared.Invalid("invalid_guest_count", "guest count must be at least 1")
	case in.MaxOccupancy > 0 && in.GuestCount > in.MaxOccupancy:
		return nil, shared.Unprocessable("occupancy_exceeded",
			"room type sleeps %d, %d guests requested", in.MaxOccupancy, in.GuestCount)
	}

	if err := in.Quote.Validate(); err != nil {
		return nil, err
	}

	return &Reservation{
		ID:         uuid.New(),
		Reference:  NewReference(),
		PropertyID: in.PropertyID,
		RoomTypeID: in.RoomTypeID,
		RatePlanID: in.RatePlanID,
		GuestID:    in.Guest.ID,
		Stay:       in.Quote.Stay,
		GuestCount: in.GuestCount,
		Status:     StatusConfirmed,
		Nights:     in.Quote.Nights,
		Total:      in.Quote.Total,
		CreatedAt:  in.Now.UTC(),
	}, nil
}

type Cancellation struct {
	At         time.Time
	Refund     shared.Money
	Refundable bool
}

func (r *Reservation) Cancel(now time.Time, refundableUntilHours int) (Cancellation, error) {
	if r.Status == StatusCancelled {
		return Cancellation{}, shared.Conflict("already_cancelled", "reservation %s is already cancelled", r.Reference)
	}

	checkIn := r.Stay.CheckIn().Time()
	if !now.Before(checkIn) {
		return Cancellation{}, shared.Conflict("stay_started", "reservation %s can no longer be cancelled online", r.Reference)
	}

	deadline := checkIn.Add(-time.Duration(refundableUntilHours) * time.Hour)
	refundable := now.Before(deadline)

	refund := shared.Zero(r.Total.Currency())
	if refundable {
		refund = r.Total
	}

	at := now.UTC()
	r.Status = StatusCancelled
	r.CancelledAt = &at

	return Cancellation{At: at, Refund: refund, Refundable: refundable}, nil
}

func (r *Reservation) IsCancelled() bool { return r.Status == StatusCancelled }

const referenceAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func NewReference() string {
	b := make([]byte, 6)
	rand.Read(b)

	for i := range b {
		b[i] = referenceAlphabet[int(b[i])%len(referenceAlphabet)]
	}
	return "ALB-" + string(b)
}
