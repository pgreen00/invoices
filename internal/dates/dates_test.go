package dates

import (
	"slices"
	"testing"
	"time"
)

func TestWeekdaysFrom(t *testing.T) {
	// 2024-03-20 is a Wednesday.
	want := []string{"2024-03-18", "2024-03-19", "2024-03-20", "2024-03-21", "2024-03-22"}
	if got := WeekdaysFrom("2024-03-20"); !slices.Equal(got, want) {
		t.Errorf("WeekdaysFrom = %v, want %v", got, want)
	}
	// Sunday belongs to the week that just ended.
	if got := WeekdaysFrom("2024-03-24")[0]; got != "2024-03-18" {
		t.Errorf("Sunday's Monday = %s", got)
	}
	if got := AddDays("2024-02-28", 2); got != "2024-03-01" { // leap year
		t.Errorf("AddDays = %s", got)
	}
}

func TestFormatRange(t *testing.T) {
	cases := [][3]string{
		{"2024-03-18", "2024-03-22", "Mar 18 – 22, 2024"},
		{"2024-03-28", "2024-04-01", "Mar 28 – Apr 1, 2024"},
		{"2024-12-30", "2025-01-03", "Dec 30, 2024 – Jan 3, 2025"},
		{"2024-03-18", "", "Mar 18, 2024"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := FormatRange(c[0], c[1]); got != c[2] {
			t.Errorf("FormatRange(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

func TestFormatters(t *testing.T) {
	if got := FormatLong("2024-03-24"); got != "Mar 24, 2024" {
		t.Errorf("FormatLong = %q", got)
	}
	if got := FormatDayDate("2024-03-18"); got != "Mon, Mar 18" {
		t.Errorf("FormatDayDate = %q", got)
	}
	if got := FormatLong("nope"); got != "" {
		t.Errorf("FormatLong(invalid) = %q", got)
	}
}

func TestTimestamps(t *testing.T) {
	stamp := Now()
	if _, err := time.Parse(time.RFC3339Nano, stamp); err != nil || len(stamp) != len("2024-03-24T14:14:00.000Z") {
		t.Fatalf("Now() = %q is not toISOString format", stamp)
	}

	local := time.Date(2024, 3, 24, 14, 14, 0, 0, time.Local)
	if got := FormatTimestamp(local.UTC().Format(Timestamp)); got != "Mar 24, 2024 at 2:14 PM" {
		t.Errorf("FormatTimestamp = %q", got)
	}
}
