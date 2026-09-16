package shared

import "fmt"

type Currency string

const EUR Currency = "EUR"

func (c Currency) valid() bool {
	if len(c) != 3 {
		return false
	}
	for _, r := range c {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

type Money struct {
	cents    int64
	currency Currency
}

func NewMoney(cents int64, currency Currency) (Money, error) {
	if !currency.valid() {
		return Money{}, Invalid("invalid_currency", "currency %q must be three uppercase letters", currency)
	}
	if cents < 0 {
		return Money{}, Invalid("negative_amount", "amount must not be negative, got %d", cents)
	}
	return Money{cents: cents, currency: currency}, nil
}

func MustMoney(cents int64, currency Currency) Money {
	m, err := NewMoney(cents, currency)
	if err != nil {
		panic(err)
	}
	return m
}

func Zero(currency Currency) Money { return Money{currency: currency} }

func (m Money) Cents() int64       { return m.cents }
func (m Money) Currency() Currency { return m.currency }
func (m Money) IsZero() bool       { return m.cents == 0 }

func (m Money) Add(o Money) (Money, error) {
	if m.currency == "" {
		return o, nil
	}
	if o.currency == "" {
		return m, nil
	}
	if m.currency != o.currency {
		return Money{}, Invalid("currency_mismatch", "cannot add %s to %s", o.currency, m.currency)
	}
	return Money{cents: m.cents + o.cents, currency: m.currency}, nil
}

func (m Money) Mul(n int64) (Money, error) {
	if n < 0 {
		return Money{}, Invalid("negative_multiplier", "multiplier must not be negative, got %d", n)
	}
	return Money{cents: m.cents * n, currency: m.currency}, nil
}

func (m Money) Equal(o Money) bool {
	return m.cents == o.cents && m.currency == o.currency
}

func (m Money) String() string {
	return fmt.Sprintf("%d.%02d %s", m.cents/100, m.cents%100, m.currency)
}
