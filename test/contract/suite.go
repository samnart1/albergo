package contract

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/samnart1/albergo/internal/app/ports"
	"github.com/samnart1/albergo/internal/domain/booking"
	"github.com/samnart1/albergo/internal/domain/pricing"
	"github.com/samnart1/albergo/internal/domain/shared"
)

type Repos struct {
	Tx           ports.TxManager
	Catalog      ports.CatalogRepository
	Rates        ports.RateRepository
	Inventory    ports.InventoryRepository
	Guests       ports.GuestRepository
	Reservations ports.ReservationRepository
}

// seed
type Fixture struct {
	PropertyID   uuid.UUID
	RoomTypeID   uuid.UUID
	RatePlanID   uuid.UUID
	MaxOccupancy int
}

type Setup func(t *testing.T) (Repos, Fixture)

func date(s string) shared.Date { return shared.MustParseDate(s) }

func stay(t *testing.T, in, out string) shared.Stay {
	t.Helper()
	s, err := shared.NewStay(date(in), date(out))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return s
}

func Run(t *testing.T, setup Setup) {
	t.Run("room type carries the property currency", func(t *testing.T) {
		repos, fx := setup(t)
		ctx := context.Background()

		rt, err := repos.Catalog.RoomType(ctx, fx.RoomTypeID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rt.PropertyID != fx.PropertyID {
			t.Errorf("property = %s, want %s", rt.PropertyID, fx.PropertyID)
		}
		if rt.Currency != shared.EUR {
			t.Errorf("currency = %q, want EUR", rt.Currency)
		}
	})

	t.Run("unknown room type is not found", func(t *testing.T) {
		repos, _ := setup(t)

		_, err := repos.Catalog.RoomType(context.Background(), uuid.New())
		if shared.KindOf(err) != shared.KindNotFound {
			t.Fatalf("kind = %v, want not found", shared.KindOf(err))
		}
	})

	t.Run("rates round trip", func(t *testing.T) {
		repos, fx := setup(t)
		ctx := context.Background()

		days := []pricing.RateDay{
			{Date: date("2026-03-14"), Price: shared.MustMoney(12_000, shared.EUR), MinStay: 2},
			{Date: date("2026-03-15"), Price: shared.MustMoney(13_500, shared.EUR), MinStay: 1, ClosedToArrival: true},
		}
		if err := repos.Rates.UpsertRates(ctx, fx.RatePlanID, days); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := repos.Rates.Rates(ctx, fx.RatePlanID, date("2026-03-14"), date("2026-03-15"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("rates = %d, want 2", len(got))
		}
		if !got[1].Price.Equal(shared.MustMoney(13_500, shared.EUR)) || !got[1].ClosedToArrival {
			t.Errorf("second day = %+v", got[1])
		}

		days[0].Price = shared.MustMoney(9_900, shared.EUR)
		if err := repos.Rates.UpsertRates(ctx, fx.RatePlanID, days[:1]); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, err = repos.Rates.Rates(ctx, fx.RatePlanID, date("2026-03-14"), date("2026-03-15"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 || !got[0].Price.Equal(shared.MustMoney(9_900, shared.EUR)) {
			t.Errorf("upsert did not overwrite: %+v", got)
		}
	})

	t.Run("reserve and release move the counters", func(t *testing.T) {
		repos, fx := setup(t)
		ctx := context.Background()
		s := stay(t, "2026-03-14", "2026-03-17")

		if err := repos.Inventory.UpsertAllotment(ctx, fx.RoomTypeID, s.CheckIn(), s.CheckOut(), 2); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := repos.Inventory.Reserve(ctx, fx.RoomTypeID, s, 1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		allotments, err := repos.Inventory.Allotments(ctx, fx.RoomTypeID, s.CheckIn(), s.CheckOut())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(allotments) != 3 {
			t.Fatalf("allotments = %d, want 3", len(allotments))
		}
		for _, a := range allotments {
			if a.Available() != 1 {
				t.Errorf("%s available = %d, want 1", a.Date, a.Available())
			}
		}

		if err := repos.Inventory.Release(ctx, fx.RoomTypeID, s, 1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		allotments, _ = repos.Inventory.Allotments(ctx, fx.RoomTypeID, s.CheckIn(), s.CheckOut())
		for _, a := range allotments {
			if a.Booked != 0 {
				t.Errorf("%s booked = %d, want 0", a.Date, a.Booked)
			}
		}
	})

	t.Run("reserve past the allotment is rejected", func(t *testing.T) {
		repos, fx := setup(t)
		ctx := context.Background()
		s := stay(t, "2026-03-14", "2026-03-16")

		if err := repos.Inventory.UpsertAllotment(ctx, fx.RoomTypeID, s.CheckIn(), s.CheckOut(), 1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := repos.Inventory.Reserve(ctx, fx.RoomTypeID, s, 1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err := repos.Inventory.Reserve(ctx, fx.RoomTypeID, s, 1)
		if shared.CodeOf(err) != "sold_out" {
			t.Fatalf("code = %q, want sold_out", shared.CodeOf(err))
		}
	})

	t.Run("reserve a night with no inventory row", func(t *testing.T) {
		repos, fx := setup(t)
		ctx := context.Background()

		if err := repos.Inventory.UpsertAllotment(ctx, fx.RoomTypeID, date("2026-03-14"), date("2026-03-16"), 5); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err := repos.Inventory.Reserve(ctx, fx.RoomTypeID, stay(t, "2026-03-14", "2026-03-17"), 1)
		if shared.CodeOf(err) != "no_inventory" {
			t.Fatalf("code = %q, want no_inventory", shared.CodeOf(err))
		}
	})

	t.Run("a rolled back transaction leaves no trace", func(t *testing.T) {
		repos, fx := setup(t)
		ctx := context.Background()
		s := stay(t, "2026-03-14", "2026-03-16")

		if err := repos.Inventory.UpsertAllotment(ctx, fx.RoomTypeID, s.CheckIn(), s.CheckOut(), 1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		boom := shared.Internal("boom", nil)
		err := repos.Tx.WithinTx(ctx, func(ctx context.Context) error {
			if err := repos.Inventory.Reserve(ctx, fx.RoomTypeID, s, 1); err != nil {
				return err
			}
			return boom
		})
		if err != boom {
			t.Fatalf("err = %v, want the sentinel back unchanged", err)
		}

		allotments, _ := repos.Inventory.Allotments(ctx, fx.RoomTypeID, s.CheckIn(), s.CheckOut())
		for _, a := range allotments {
			if a.Booked != 0 {
				t.Errorf("%s booked = %d after rollback, want 0", a.Date, a.Booked)
			}
		}
	})

	t.Run("guest upsert reuses the row", func(t *testing.T) {
		repos, _ := setup(t)
		ctx := context.Background()

		first, err := booking.NewGuest("sam@example.com", "Sam T", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		stored, err := repos.Guests.Upsert(ctx, first)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		second, _ := booking.NewGuest("SAM@example.com", "Samuel T", "+39 333")
		again, err := repos.Guests.Upsert(ctx, second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if again.ID != stored.ID {
			t.Errorf("id = %s, want the existing %s", again.ID, stored.ID)
		}
	})

	t.Run("reservation round trip and cancel", func(t *testing.T) {
		repos, fx := setup(t)
		ctx := context.Background()
		s := stay(t, "2026-03-14", "2026-03-16")

		guest, _ := booking.NewGuest("sam@example.com", "Sam T", "")
		guest, err := repos.Guests.Upsert(ctx, guest)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rates := []pricing.RateDay{
			{Date: date("2026-03-14"), Price: shared.MustMoney(10_000, shared.EUR), MinStay: 1},
			{Date: date("2026-03-15"), Price: shared.MustMoney(12_000, shared.EUR), MinStay: 1},
		}
		quote, err := pricing.Calculate(s, rates)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		res, err := booking.New(booking.NewInput{
			PropertyID:   fx.PropertyID,
			RoomTypeID:   fx.RoomTypeID,
			RatePlanID:   fx.RatePlanID,
			Guest:        guest,
			Quote:        quote,
			GuestCount:   2,
			MaxOccupancy: fx.MaxOccupancy,
			Now:          time.Date(2026, time.February, 1, 12, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := repos.Reservations.Create(ctx, res); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		loaded, err := repos.Reservations.ByReference(ctx, res.Reference)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !loaded.Total.Equal(shared.MustMoney(22_000, shared.EUR)) {
			t.Errorf("total = %s, want 220.00 EUR", loaded.Total)
		}
		if len(loaded.Nights) != 2 || !loaded.Nights[1].Price.Equal(shared.MustMoney(12_000, shared.EUR)) {
			t.Errorf("nights did not round trip: %+v", loaded.Nights)
		}
		if loaded.Status != booking.StatusConfirmed || loaded.CancelledAt != nil {
			t.Errorf("status = %q, cancelled = %v", loaded.Status, loaded.CancelledAt)
		}

		if _, err := loaded.Cancel(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), 48); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := repos.Reservations.Cancel(ctx, loaded); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := repos.Reservations.Cancel(ctx, loaded); shared.CodeOf(err) != "already_cancelled" {
			t.Fatalf("code = %q, want already_cancelled", shared.CodeOf(err))
		}

		reloaded, err := repos.Reservations.ByID(ctx, res.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reloaded.IsCancelled() || reloaded.CancelledAt == nil {
			t.Error("cancellation did not persist")
		}
	})
}
