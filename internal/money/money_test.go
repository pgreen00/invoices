package money

import "testing"

func TestParseCents(t *testing.T) {
	cases := map[string]int64{
		"165":       16500,
		"$1,234.56": 123456,
		"247.50":    24750,
		"":          0,
		"garbage":   0,
		" 12.345 ":  1235,
		"1.005":     100, // 100.49999… in binary, as in JavaScript
		"Infinity":  0,
	}
	for input, want := range cases {
		if got := ParseCents(input); got != want {
			t.Errorf("ParseCents(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestParseHours(t *testing.T) {
	cases := map[string]float64{
		"8":     8,
		"7.5":   7.5,
		"0.125": 0.13,
		"-1":    0,
		"":      0,
		"x":     0,
	}
	for input, want := range cases {
		if got := ParseHours(input); got != want {
			t.Errorf("ParseHours(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestFormatCents(t *testing.T) {
	cases := map[int64]string{
		622875:    "$6,228.75",
		0:         "$0.00",
		-500:      "-$5.00",
		5:         "$0.05",
		100000000: "$1,000,000.00",
	}
	for input, want := range cases {
		if got := FormatCents(input); got != want {
			t.Errorf("FormatCents(%d) = %q, want %q", input, got, want)
		}
	}
}

func TestComputeTotals(t *testing.T) {
	totals := ComputeTotals([]Line{
		{Hours: 8, RateCents: 16500},
		{Hours: 7.5, RateCents: 16500},
		{Hours: 2, RateCents: 24750},
	}, 2500, 1000)

	if totals.Hours != 17.5 {
		t.Errorf("hours = %v", totals.Hours)
	}
	if want := int64(132000 + 123750 + 49500); totals.SubtotalCents != want {
		t.Errorf("subtotal = %d, want %d", totals.SubtotalCents, want)
	}
	if want := totals.SubtotalCents + 2500 - 1000; totals.TotalCents != want {
		t.Errorf("total = %d, want %d", totals.TotalCents, want)
	}
}
