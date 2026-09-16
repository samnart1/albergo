package shared

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		in      string
		wantErr bool
	}{
		{in: "2026-03-14"},
		{in: "2026-02-29", wantErr: true},
		{in: "14-03-2026", wantErr: true},
		{in: "2026-03-14T00:00:00Z", wantErr: true},
		{in: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseDate(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %s", got)
				}
				if CodeOf(err) != "invalid_date" {
					t.Fatalf("code = %q, want invalid_date", CodeOf(err))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tt.in {
				t.Errorf("String() = %q, want %q", got.String(), tt.in)
			}
		})
	}
}

func TestDateAddDays(t *testing.T) {
	tests := []struct {
		name string
		in   string
		days int
		want string
	}{
		{name: "across a month", in: "2026-03-30", days: 3, want: "2026-04-02"},
		{name: "across a year", in: "2026-12-31", days: 1, want: "2027-01-01"},
		{name: "leap day", in: "2028-02-28", days: 1, want: "2028-02-29"},
		{name: "backwards", in: "2026-03-01", days: -1, want: "2026-02-28"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MustParseDate(tt.in).AddDays(tt.days); got.String() != tt.want {
				t.Errorf("AddDays(%d) = %s, want %s", tt.days, got, tt.want)
			}
		})
	}
}

func TestDateDaysUntilAcrossDSTChange(t *testing.T) {
	// 29 March 2026
	from := MustParseDate("2026-03-27")
	to := MustParseDate("2026-03-30")

	if got := from.DaysUntil(to); got != 3 {
		t.Errorf("DaysUntil = %d, want 3", got)
	}
}

func TestDateNormalises(t *testing.T) {
	if got := NewDate(2026, time.April, 31); got.String() != "2026-05-01" {
		t.Errorf("NewDate = %s, want 2026-05-01", got)
	}
}

func TestDateJSONRoundTrip(t *testing.T) {
	in := struct {
		CheckIn Date `json:"check_in"`
	}{CheckIn: MustParseDate("2026-03-14")}

	encoded, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(encoded) != `{"check_in":"2026-03-14"}` {
		t.Fatalf("encoded = %s", encoded)
	}

	var out struct {
		CheckIn Date `json:"check_in"`
	}
	if err := json.Unmarshal(encoded, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.CheckIn != in.CheckIn {
		t.Errorf("round trip = %s, want %s", out.CheckIn, in.CheckIn)
	}
}
