package booking

import (
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/samnart1/albergo/internal/domain/pricing"
	"github.com/samnart1/albergo/internal/domain/shared"
)

func testQuote(t *testing.T) pricing.Quote {
	t.Helper()

	stay, err := shared.NewStay(shared.MustParseDate("2026-03-14"), shared.MustParseDate("2026-03-16"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rates := []pricing.RateDay{
		{Date: shared.MustParseDate("2026-03-14"), Price: shared.MustMoney(10_000, shared.EUR), MinStay: 1},
		{Date: shared.MustParseDate("2026-03-15"), Price: shared.MustMoney(10_000, shared.EUR), MinStay: 1},
	}

	quote, err := pricing.Calculate(stay, rates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return quote
}

func testInput(t *testing.T) NewInput {
	t.Helper()

	guest, err := NewGuest("Sam <SAM@example.com>", "Sam T", "+39 333 000 0000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return NewInput{
		PropertyID:   uuid.New(),
		RoomTypeID:   uuid.New(),
		RatePlanID:   uuid.New(),
		Guest:        guest,
		Quote:        testQuote(t),
		GuestCount:   2,
		MaxOccupancy: 2,
		Now:          time.Date(2026, time.February, 1, 12, 0, 0, 0, time.UTC),
	}
}

func TestNewGuestNormalisesEmail(t *testing.T) {
	guest, err := NewGuest("  Sam <SAM@Example.com>  ", " Sam T ", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if guest.Email != "sam@example.com" {
		t.Errorf("email = %q, want sam@example.com", guest.Email)
	}
	if guest.FullName != "Sam T" {
		t.Errorf("name = %q, want %q", guest.FullName, "Sam T")
	}
}

func TestNewGuestRejectsBadEmail(t *testing.T) {
	if _, err := NewGuest("not-an-email", "Sam T", ""); shared.CodeOf(err) != "invalid_email" {
		t.Fatalf("code = %q, want invalid_email", shared.CodeOf(err))
	}
}

func TestNewReservation(t *testing.T) {
	in := testInput(t)

	res, err := New(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Status != StatusConfirmed {
		t.Errorf("status = %q, want confirmed", res.Status)
	}
	if !res.Total.Equal(shared.MustMoney(20_000, shared.EUR)) {
		t.Errorf("total = %s, want 200.00 EUR", res.Total)
	}
	if !strings.HasPrefix(res.Reference, "ALB-") || len(res.Reference) != 10 {
		t.Errorf("reference = %q, want ALB- plus 6 characters", res.Reference)
	}
	if res.CancelledAt != nil {
		t.Error("new reservation must not be cancelled")
	}
}

func TestNewReservationValidation(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(in *NewInput)
		wantCode string
	}{
		{name: "no property", mutate: func(in *NewInput) { in.PropertyID = uuid.Nil() }, wantCode: "missing_identifier"},
		{name: "no guests", mutate: func(in *NewInput) { in.GuestCount = 0 }, wantCode: "invalid_guest_count"},
		{name: "over occupancy", mutate: func(in *NewInput) { in.GuestCount = 3 }, wantCode: "occupancy_exceeded"},
		{
			name:     "tampered quote",
			mutate:   func(in *NewInput) { in.Quote.Total = shared.MustMoney(1, shared.EUR) },
			wantCode: "invalid_quote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := testInput(t)
			tt.mutate(&in)

			if _, err := New(in); shared.CodeOf(err) != tt.wantCode {
				t.Fatalf("code = %q, want %q", shared.CodeOf(err), tt.wantCode)
			}
		})
	}
}

func TestCancel(t *testing.T) {
	const refundableUntilHours = 48

	tests := []struct {
		name           string
		now            time.Time
		wantRefundable bool
		wantRefund     int64
	}{
		{
			name:           "well before the deadline",
			now:            time.Date(2026, time.March, 1, 9, 0, 0, 0, time.UTC),
			wantRefundable: true,
			wantRefund:     20_000,
		},
		{
			name:       "one minute after the deadline",
			now:        time.Date(2026, time.March, 12, 0, 1, 0, 0, time.UTC),
			wantRefund: 0,
		},
		{
			name:       "the day before check in",
			now:        time.Date(2026, time.March, 13, 18, 0, 0, 0, time.UTC),
			wantRefund: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := New(testInput(t))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			cancellation, err := res.Cancel(tt.now, refundableUntilHours)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cancellation.Refundable != tt.wantRefundable {
				t.Errorf("refundable = %v, want %v", cancellation.Refundable, tt.wantRefundable)
			}
			if cancellation.Refund.Cents() != tt.wantRefund {
				t.Errorf("refund = %d, want %d", cancellation.Refund.Cents(), tt.wantRefund)
			}
			if !res.IsCancelled() || res.CancelledAt == nil {
				t.Error("reservation was not cancelled")
			}
		})
	}
}

func TestCancelRejected(t *testing.T) {
	t.Run("after check in", func(t *testing.T) {
		res, err := New(testInput(t))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		now := time.Date(2026, time.March, 14, 10, 0, 0, 0, time.UTC)
		if _, err := res.Cancel(now, 48); shared.CodeOf(err) != "stay_started" {
			t.Fatalf("code = %q, want stay_started", shared.CodeOf(err))
		}
		if res.IsCancelled() {
			t.Error("rejected cancellation must not mutat")
		}
	})

	t.Run("twice", func(t *testing.T) {
		res, err := New(testInput(t))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		now := time.Date(2026, time.March, 1, 9, 0, 0, 0, time.UTC)
		if _, err := res.Cancel(now, 48); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := res.Cancel(now, 48); shared.CodeOf(err) != "already_cancelled" {
			t.Fatalf("code = %q, want already_cancelled", shared.CodeOf(err))
		}
	})
}
