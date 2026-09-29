package handlers

import (
	"testing"
	"time"
)

func TestAddMonthsLikeDotNet(t *testing.T) {
	cases := []struct {
		in, want string
		months   int
	}{
		{"2026-01-31T10:00:00Z", "2026-02-28T10:00:00Z", 1},
		{"2028-01-31T10:00:00Z", "2028-02-29T10:00:00Z", 1}, // năm nhuận
		{"2026-08-31T00:00:00Z", "2027-02-28T00:00:00Z", 6},
		{"2026-03-15T08:30:00Z", "2027-03-15T08:30:00Z", 12},
	}
	for _, c := range cases {
		in, _ := time.Parse(time.RFC3339, c.in)
		if got := addMonths(in, c.months).Format(time.RFC3339); got != c.want {
			t.Errorf("addMonths(%s, %d) = %s, want %s", c.in, c.months, got, c.want)
		}
	}
}
