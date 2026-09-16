package app

import (
	"context"
	"uuid"

	"github.com/samnart1/albergo/internal/app/ports"
	"github.com/samnart1/albergo/internal/domain/booking"
	"github.com/samnart1/albergo/internal/domain/pricing"
	"github.com/samnart1/albergo/internal/domain/shared"
)

type ReservationService struct {
	tx           ports.TxManager
	catalog      ports.CatalogRepository
	rates        ports.RateRepository
	inventory    ports.InventoryRepository
	guests       ports.GuestRepository
	reservations ports.ReservationRepository
	idempotency  ports.IdempotencyRepository
	outbox       ports.OutboxRepository
	clock        shared.Clock
}

type ReserveInput struct {
	IdempotencyKey string
	RequestHash    string
	PropertyID     uuid.UUID
	RoomTypeID     uuid.UUID
	RatePlanID     uuid.UUID
	Stay           shared.Stay
	Guests         int
	Email          string
	FullName       string
	Phone          string
}

type ReserveResult struct {
	Reservation *booking.Reservation
	Replayed    bool
}

const unitsPerReservation = 1

func (s *ReservationService) Reserve(ctx context.Context, in ReserveInput) (ReserveResult, error) {
	var out ReserveResult

	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		claim, err := s.idempotency.Claim(ctx, in.IdempotencyKey, in.RequestHash)
		if err != nil {
			return err
		}

		if !claim.Fresh {
			if claim.RequestHash != in.RequestHash {
				return shared.Conflict("idempotency_key_reused",
					"that idempotency key was already used for a different request")
			}
			if claim.ReservationID == nil {
				return shared.Conflict("request_in_progress", "an identical request is still being processed")
			}

			res, err := s.reservations.ByID(ctx, *claim.ReservationID)
			if err != nil {
				return err
			}
			out = ReserveResult{Reservation: res, Replayed: true}
			return nil
		}

		res, guest, err := s.book(ctx, in)
		if err != nil {
			return err
		}

		if err := s.idempotency.Record(ctx, in.IdempotencyKey, res.ID); err != nil {
			return err
		}
		if err := s.outbox.Publish(ctx, ports.Event{
			AggregateID: res.ID,
			Type:        EventReservationConfirmed,
			Payload:     newReservationEvent(res, guest),
		}); err != nil {
			return err
		}

		out = ReserveResult{Reservation: res}
		return nil
	})
	if err != nil {
		return ReserveResult{}, err
	}
	return out, nil
}

func (s *ReservationService) book(ctx context.Context, in ReserveInput) (*booking.Reservation, booking.Guest, error) {
	roomType, err := s.catalog.RoomType(ctx, in.RoomTypeID)
	if err != nil {
		return nil, booking.Guest{}, err
	}
	// comes with the bdy
	if roomType.PropertyID != in.PropertyID {
		return nil, booking.Guest{}, shared.NotFound("room_type_not_found",
			"room type %s does not belong to that property", in.RoomTypeID)
	}

	ratePlan, err := s.catalog.RatePlan(ctx, in.RatePlanID)
	if err != nil {
		return nil, booking.Guest{}, err
	}
	if ratePlan.RoomTypeID != in.RoomTypeID {
		return nil, booking.Guest{}, shared.Invalid("rate_plan_mismatch",
			"rate plan %s is not sold for that room type", in.RatePlanID)
	}

	rates, err := s.rates.Rates(ctx, ratePlan.ID, in.Stay.CheckIn(), in.Stay.CheckOut())
	if err != nil {
		return nil, booking.Guest{}, err
	}

	quote, err := pricing.Calculate(in.Stay, rates)
	if err != nil {
		return nil, booking.Guest{}, err
	}

	guest, err := booking.NewGuest(in.Email, in.FullName, in.Phone)
	if err != nil {
		return nil, booking.Guest{}, err
	}
	guest, err = s.guests.Upsert(ctx, guest)
	if err != nil {
		return nil, booking.Guest{}, err
	}

	res, err := booking.New(booking.NewInput{
		PropertyID:   roomType.PropertyID,
		RoomTypeID:   roomType.ID,
		RatePlanID:   ratePlan.ID,
		Guest:        guest,
		Quote:        quote,
		GuestCount:   in.Guests,
		MaxOccupancy: roomType.MaxOccupancy,
		Now:          s.clock.Now(),
	})
	if err != nil {
		return nil, booking.Guest{}, err
	}

	if err := s.inventory.Reserve(ctx, roomType.ID, in.Stay, unitsPerReservation); err != nil {
		return nil, booking.Guest{}, err
	}
	if err := s.reservations.Create(ctx, res); err != nil {
		return nil, booking.Guest{}, err
	}
	return res, guest, nil
}

func (s *ReservationService) ByReference(ctx context.Context, reference string) (*booking.Reservation, error) {
	return s.reservations.ByReference(ctx, reference)
}

type CancelResult struct {
	Reservation  *booking.Reservation
	Cancellation booking.Cancellation
}

func (s *ReservationService) Cancel(ctx context.Context, reference string) (CancelResult, error) {
	var out CancelResult

	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		res, err := s.reservations.ByReference(ctx, reference)
		if err != nil {
			return err
		}

		ratePlan, err := s.catalog.RatePlan(ctx, res.RatePlanID)
		if err != nil {
			return err
		}

		cancellation, err := res.Cancel(s.clock.Now(), ratePlan.RefundableUntilHours)
		if err != nil {
			return err
		}

		if err := s.reservations.Cancel(ctx, res); err != nil {
			return err
		}
		if err := s.inventory.Release(ctx, res.RoomTypeID, res.Stay, unitsPerReservation); err != nil {
			return err
		}

		guest, err := s.guests.ByID(ctx, res.GuestID)
		if err != nil {
			return err
		}

		event := newReservationEvent(res, guest)
		event.RefundCents = cancellation.Refund.Cents()
		event.Refundable = cancellation.Refundable

		if err := s.outbox.Publish(ctx, ports.Event{
			AggregateID: res.ID,
			Type:        EventReservationCancelled,
			Payload:     event,
		}); err != nil {
			return err
		}

		out = CancelResult{Reservation: res, Cancellation: cancellation}
		return nil
	})
	if err != nil {
		return CancelResult{}, err
	}
	return out, nil
}
