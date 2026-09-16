package shared

import "testing"

func TestNewMoney(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		currency Currency
		wantCode string
	}{
		{name: "valid", cents: 12500, currency: EUR},
		{name: "zero is valid", cents: 0, currency: EUR},
		{name: "negative", cents: -1, currency: EUR, wantCode: "negative_amount"},
		{name: "empty currency", cents: 100, currency: "", wantCode: "invalid_currency"},
		{name: "lowercase currency", cents: 100, currency: "eur", wantCode: "invalid_currency"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMoney(tt.cents, tt.currency)

			if tt.wantCode != "" {
				if err == nil {
					t.Fatalf("expected error %q, got money %v", tt.wantCode, got)
				}
				if code := CodeOf(err); code != tt.wantCode {
					t.Fatalf("code = %q, want %q", code, tt.wantCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Cents() != tt.cents {
				t.Errorf("cents = %d, want %d", got.Cents(), tt.cents)
			}
		})
	}
}

func TestMoneyAdd(t *testing.T) {
	t.Run("same currency", func(t *testing.T) {
		got, err := MustMoney(10_000, EUR).Add(MustMoney(2_550, EUR))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Cents() != 12_550 {
			t.Errorf("cents = %d, want 12550", got.Cents())
		}
	})

	t.Run("zero value is the identity", func(t *testing.T) {
		var acc Money
		got, err := acc.Add(MustMoney(9_900, EUR))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.Equal(MustMoney(9_900, EUR)) {
			t.Errorf("got %v, want 99.00 EUR", got)
		}
	})

	t.Run("currency mismatch", func(t *testing.T) {
		_, err := MustMoney(100, EUR).Add(MustMoney(100, "CHF"))
		if CodeOf(err) != "currency_mismatch" {
			t.Fatalf("code = %q, want currency_mismatch", CodeOf(err))
		}
	})
}

func TestMoneyString(t *testing.T) {
	if got := MustMoney(12_505, EUR).String(); got != "125.05 EUR" {
		t.Errorf("String() = %q, want %q", got, "125.05 EUR")
	}
}
