package pricing

import (
	"fmt"
	"github.com/samnart1/albergo/internal/domain/shared"
)

type RateDay struct {
	Date              shared.Date
	Price             shared.Money
	MinStay           int
	ClosedToArrival   bool
	ClosedToDeparture bool
}

type Night struct {
	Date  shared.Date
	Price shared.Money
}

type Quote struct {
	Stay   shared.Stay
	Nights []Night
	Total  shared.Money
}

func Calculate(stay shared.Stay, rates []RateDay) (Quote, error) {
	byDate := make(map[shared.Date]RateDay, len(rates))
	for _, r := range rates {
		byDate[r.Date] = r
	}

	arrival := stay.CheckIn()
	if r, ok := byDate[arrival]; ok && r.ClosedToArrival {
		return Quote{}, shared.Unprocessable("closed_to_arrival", "arrivals are not accepted on %s", arrival)
	}

	departure := stay.CheckOut()
	if r, ok := byDate[departure]; ok && r.ClosedToDeparture {
		return Quote{}, shared.Unprocessable("closed_to_departure", "departures are not accepted on %s", departure)
	}

	nights := make([]Night, 0, stay.Nights())
	var total shared.Money

	for _, d := range stay.Dates() {
		rate, ok := byDate[d]
		if !ok {
			return Quote{}, shared.Unprocessable("rate_unavailable", "no rate is published for %s", d)
		}

		if rate.Price.Currency() == "" {
			return Quote{}, shared.Internal("invalid_rate",
				fmt.Errorf("rate published for %s has no currency", d))
		}

		sum, err := total.Add(rate.Price)
		if err != nil {
			return Quote{}, err
		}
		total = sum
		nights = append(nights, Night{Date: d, Price: rate.Price})
	}

	if minStay := byDate[arrival].MinStay; minStay > stay.Nights() {
		return Quote{}, shared.Unprocessable("min_stay_not_met",
			"arrival on %s requires at least %d nights, %d requested", arrival, minStay, stay.Nights())
	}

	return Quote{Stay: stay, Nights: nights, Total: total}, nil
}

func (q Quote) Validate() error {
	if len(q.Nights) != q.Stay.Nights() {
		return shared.Invalid("invalid_quote", "quote covers %d nights but the stay is %d", len(q.Nights), q.Stay.Nights())
	}

	var total shared.Money
	for _, n := range q.Nights {
		if n.Price.Currency() == "" {
			return shared.Invalid("invalid_quote", "the night on %s has no price", n.Date)
		}
		sum, err := total.Add(n.Price)
		if err != nil {
			return err
		}
		total = sum
	}

	if !total.Equal(q.Total) {
		return shared.Invalid("invalid_quote", "quote total %s does not match the sum of nights %s", q.Total, total)
	}
	return nil
}
