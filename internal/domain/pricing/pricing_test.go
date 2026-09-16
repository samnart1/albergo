package pricing

import (
	"testing"

	"github.com/samnart1/albergo/internal/domain/shared"
)

func mustStay(t *testing.T, in, out string) shared.Stay {
	t.Helper()
	s, err := shared.NewStay(shared.MustParseDate(in), shared.MustParseDate(out))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return s
}

func rateDays(from string, nights int, cents int64) []RateDay {
	days := make([]RateDay, 0, nights)
	d := shared.MustParseDate(from)
	for i := 0; i < nights; i++ {
		days = append(days, RateDay{
			Date:    d,
			Price:   shared.MustMoney(cents, shared.EUR),
			MinStay: 1,
		})
		d = d.AddDays(1)
	}
	return days
}

func TestCalculate(t *testing.T) {
	stay := mustStay(t, "2026-03-14", "2026-03-17")

	quote, err := Calculate(stay, rateDays("2026-03-14", 4, 12_000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(quote.Nights) != 3 {
		t.Fatalf("nights = %d, want 3", len(quote.Nights))
	}
	if !quote.Total.Equal(shared.MustMoney(36_000, shared.EUR)) {
		t.Errorf("total = %s, want 360.00 EUR", quote.Total)
	}
	if err := quote.Validate(); err != nil {
		t.Errorf("valid quote failed validation: %v", err)
	}
}

func TestCalculateVaryingRates(t *testing.T) {
	stay := mustStay(t, "2026-03-14", "2026-03-16")

	rates := []RateDay{
		{Date: shared.MustParseDate("2026-03-14"), Price: shared.MustMoney(10_000, shared.EUR), MinStay: 1},
		{Date: shared.MustParseDate("2026-03-15"), Price: shared.MustMoney(15_500, shared.EUR), MinStay: 1},
	}

	quote, err := Calculate(stay, rates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !quote.Total.Equal(shared.MustMoney(25_500, shared.EUR)) {
		t.Errorf("total = %s, want 255.00 EUR", quote.Total)
	}
}

func TestCalculateRestrictions(t *testing.T) {
	tests := []struct {
		name     string
		in, out  string
		mutate   func(days []RateDay)
		wantCode string
	}{
		{
			name: "rate published without a price",
			in:   "2026-03-14", out: "2026-03-17",
			mutate:   func(days []RateDay) { days[1].Price = shared.Money{} },
			wantCode: "invalid_rate",
		},
		{
			name: "rate in a different currency",
			in:   "2026-03-14", out: "2026-03-17",
			mutate:   func(days []RateDay) { days[1].Price = shared.MustMoney(12_000, "CHF") },
			wantCode: "currency_mismatch",
		},
		{
			name: "closed to arrival",
			in:   "2026-03-14", out: "2026-03-17",
			mutate:   func(days []RateDay) { days[0].ClosedToArrival = true },
			wantCode: "closed_to_arrival",
		},
		{
			name: "closed to departure",
			in:   "2026-03-14", out: "2026-03-17",
			mutate:   func(days []RateDay) { days[3].ClosedToDeparture = true },
			wantCode: "closed_to_departure",
		},
		{
			name: "minimum stay not met",
			in:   "2026-03-14", out: "2026-03-16",
			mutate:   func(days []RateDay) { days[0].MinStay = 3 },
			wantCode: "min_stay_not_met",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			days := rateDays("2026-03-14", 4, 12_000)
			tt.mutate(days)

			_, err := Calculate(mustStay(t, tt.in, tt.out), days)
			if shared.CodeOf(err) != tt.wantCode {
				t.Fatalf("code = %q, want %q", shared.CodeOf(err), tt.wantCode)
			}
		})
	}
}

func TestCalculateUnpublishedNight(t *testing.T) {
	stay := mustStay(t, "2026-03-14", "2026-03-17")

	// only the first two nights published.
	_, err := Calculate(stay, rateDays("2026-03-14", 2, 12_000))
	if shared.CodeOf(err) != "rate_unavailable" {
		t.Fatalf("code = %q, want rate_unavailable", shared.CodeOf(err))
	}
}

func TestQuoteValidateRejectsTamperedTotal(t *testing.T) {
	stay := mustStay(t, "2026-03-14", "2026-03-16")

	quote, err := Calculate(stay, rateDays("2026-03-14", 3, 10_000))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	quote.Total = shared.MustMoney(1, shared.EUR)
	if shared.CodeOf(quote.Validate()) != "invalid_quote" {
		t.Fatal("tampered total passed validation")
	}
}
