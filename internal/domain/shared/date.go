package shared

import "time"

type Date struct {
	year  int
	month time.Month
	day   int
}

func NewDate(year int, month time.Month, day int) Date {
	return DateOf(time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
}

func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{year: y, month: m, day: d}
}

func ParseDate(s string) (Date, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return Date{}, Invalid("invalid_date", "date %q must be formatted as YYYY-MM-DD", s)
	}
	return DateOf(t), nil
}

func MustParseDate(s string) Date {
	d, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func (d Date) IsZero() bool { return d == Date{} }

func (d Date) Time() time.Time {
	return time.Date(d.year, d.month, d.day, 0, 0, 0, 0, time.UTC)
}

func (d Date) String() string { return d.Time().Format(time.DateOnly) }

func (d Date) AddDays(n int) Date { return DateOf(d.Time().AddDate(0, 0, n)) }

func (d Date) Compare(o Date) int { return d.Time().Compare(o.Time()) }

func (d Date) Before(o Date) bool { return d.Compare(o) < 0 }
func (d Date) After(o Date) bool  { return d.Compare(o) > 0 }

// UTC midnights
func (d Date) DaysUntil(o Date) int {
	return int(o.Time().Sub(d.Time()) / (24 * time.Hour))
}

func (d Date) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

func (d *Date) UnmarshalText(b []byte) error {
	parsed, err := ParseDate(string(b))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
