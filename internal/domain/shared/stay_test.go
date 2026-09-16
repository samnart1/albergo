package shared

import (
	"slices"
	"testing"
)

func TestNewStay(t *testing.T) {
	tests := []struct {
		name     string
		in, out  string
		wantCode string
	}{
		{name: "one night", in: "2026-03-14", out: "2026-03-15"},
		{name: "same day", in: "2026-03-14", out: "2026-03-14", wantCode: "invalid_stay"},
		{name: "reversed", in: "2026-03-15", out: "2026-03-14", wantCode: "invalid_stay"},
		{name: "too long", in: "2026-03-01", out: "2026-04-15", wantCode: "stay_too_long"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStay(MustParseDate(tt.in), MustParseDate(tt.out))

			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if CodeOf(err) != tt.wantCode {
				t.Fatalf("code = %q, want %q", CodeOf(err), tt.wantCode)
			}
		})
	}
}

func TestStayDatesExcludesCheckOut(t *testing.T) {
	stay, err := NewStay(MustParseDate("2026-03-14"), MustParseDate("2026-03-17"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stay.Nights() != 3 {
		t.Fatalf("Nights() = %d, want 3", stay.Nights())
	}

	want := []string{"2026-03-14", "2026-03-15", "2026-03-16"}
	got := make([]string, 0, 3)
	for _, d := range stay.Dates() {
		got = append(got, d.String())
	}

	if !slices.Equal(got, want) {
		t.Errorf("Dates() = %v, want %v", got, want)
	}
}

func TestStayOverlaps(t *testing.T) {
	mustStay := func(in, out string) Stay {
		s, err := NewStay(MustParseDate(in), MustParseDate(out))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return s
	}

	base := mustStay("2026-03-14", "2026-03-18")

	tests := []struct {
		name  string
		other Stay
		want  bool
	}{
		{name: "identical", other: mustStay("2026-03-14", "2026-03-18"), want: true},
		{name: "contained", other: mustStay("2026-03-15", "2026-03-16"), want: true},
		{name: "overlapping tail", other: mustStay("2026-03-17", "2026-03-20"), want: true},
		{name: "checks in as the other leaves", other: mustStay("2026-03-18", "2026-03-20")},
		{name: "leaves as the other checks in", other: mustStay("2026-03-10", "2026-03-14")},
		{name: "disjoint", other: mustStay("2026-04-01", "2026-04-03")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := base.Overlaps(tt.other); got != tt.want {
				t.Errorf("Overlaps = %v, want %v", got, tt.want)
			}
		})
	}
}
