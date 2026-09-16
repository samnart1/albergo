package inventory

import "github.com/samnart1/albergo/internal/domain/shared"

type Allotment struct {
	Date      shared.Date
	Allotment int
	Booked    int
}

func (a Allotment) Available() int {
	if a.Booked > a.Allotment {
		return 0
	}
	return a.Allotment - a.Booked
}

func Check(stay shared.Stay, allotments []Allotment, units int) error {
	if units < 1 {
		return shared.Invalid("invalid_units", "units must be at least 1, got %d", units)
	}

	byDate := make(map[shared.Date]Allotment, len(allotments))
	for _, a := range allotments {
		byDate[a.Date] = a
	}

	for _, d := range stay.Dates() {
		a, ok := byDate[d]
		if !ok {
			return shared.Unprocessable("no_inventory", "no inventory is published for %s", d)
		}
		if a.Available() < units {
			return shared.Conflict("sold_out", "sold out on %s", d)
		}
	}
	return nil
}
