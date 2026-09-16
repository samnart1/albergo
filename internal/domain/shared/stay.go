package shared

import "fmt"

const MaxStayNights = 30

type Stay struct {
	checkIn  Date
	checkOut Date
}

func NewStay(checkIn, checkOut Date) (Stay, error) {
	if checkIn.IsZero() || checkOut.IsZero() {
		return Stay{}, Invalid("invalid_stay", "check in and check out dates are required")
	}
	if !checkOut.After(checkIn) {
		return Stay{}, Invalid("invalid_stay", "check out %s must be after check in %s", checkOut, checkIn)
	}
	if nights := checkIn.DaysUntil(checkOut); nights > MaxStayNights {
		return Stay{}, Invalid("stay_too_long", "stay of %d nights exceeds the maximum of %d", nights, MaxStayNights)
	}
	return Stay{checkIn: checkIn, checkOut: checkOut}, nil
}

func (s Stay) CheckIn() Date  { return s.checkIn }
func (s Stay) CheckOut() Date { return s.checkOut }
func (s Stay) Nights() int    { return s.checkIn.DaysUntil(s.checkOut) }

func (s Stay) Dates() []Date {
	dates := make([]Date, 0, s.Nights())
	for d := s.checkIn; d.Before(s.checkOut); d = d.AddDays(1) {
		dates = append(dates, d)
	}
	return dates
}

func (s Stay) Contains(d Date) bool {
	return !d.Before(s.checkIn) && d.Before(s.checkOut)
}

func (s Stay) Overlaps(o Stay) bool {
	return s.checkIn.Before(o.checkOut) && o.checkIn.Before(s.checkOut)
}

func (s Stay) String() string {
	return fmt.Sprintf("%s to %s", s.checkIn, s.checkOut)
}
