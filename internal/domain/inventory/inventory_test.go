package inventory

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

func TestAvailable(t *testing.T) {
	tests := []struct {
		name      string
		allotment Allotment
		want      int
	}{
		{name: "free", allotment: Allotment{Allotment: 5}, want: 5},
		{name: "partly sold", allotment: Allotment{Allotment: 5, Booked: 3}, want: 2},
		{name: "sold out", allotment: Allotment{Allotment: 5, Booked: 5}, want: 0},
		{name: "oversold clamps at zero", allotment: Allotment{Allotment: 5, Booked: 7}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.allotment.Available(); got != tt.want {
				t.Errorf("Available() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	stay := mustStay(t, "2026-03-14", "2026-03-17")

	full := []Allotment{
		{Date: shared.MustParseDate("2026-03-14"), Allotment: 2},
		{Date: shared.MustParseDate("2026-03-15"), Allotment: 2},
		{Date: shared.MustParseDate("2026-03-16"), Allotment: 2},
	}

	tests := []struct {
		name       string
		allotments []Allotment
		units      int
		wantCode   string
	}{
		{name: "available", allotments: full, units: 1},
		{name: "exactly enough", allotments: full, units: 2},
		{name: "not enough units", allotments: full, units: 3, wantCode: "sold_out"},
		{
			name: "middle night sold out",
			allotments: []Allotment{
				full[0],
				{Date: shared.MustParseDate("2026-03-15"), Allotment: 2, Booked: 2},
				full[2],
			},
			units:    1,
			wantCode: "sold_out",
		},
		{name: "night not published", allotments: full[:2], units: 1, wantCode: "no_inventory"},
		{name: "zero units", allotments: full, units: 0, wantCode: "invalid_units"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Check(stay, tt.allotments, tt.units)

			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if shared.CodeOf(err) != tt.wantCode {
				t.Fatalf("code = %q, want %q", shared.CodeOf(err), tt.wantCode)
			}
		})
	}
}
