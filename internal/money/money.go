// Package money keeps every monetary value as integer cents. Floats only
// appear at the edges: parsing user input and formatting output.
package money

import (
	"strconv"
	"strings"

	"invoices/internal/jscompat"
)

// ParseCents turns "$1,234.56" into 123456. Anything unparseable is 0.
func ParseCents(input string) int64 {
	cleaned := strings.Map(func(r rune) rune {
		if r == '$' || r == ',' || jscompat.IsSpace(r) {
			return -1
		}
		return r
	}, input)
	if cleaned == "" {
		return 0
	}
	value, ok := jscompat.FiniteNumber(cleaned)
	if !ok {
		return 0
	}
	return int64(jscompat.Round(value * 100))
}

// ParseHours parses a non-negative hour count, keeping two decimals so
// 15-minute increments stay exact. Anything unparseable or negative is 0.
func ParseHours(input string) float64 {
	cleaned := jscompat.Trim(input)
	if cleaned == "" {
		return 0
	}
	value, ok := jscompat.FiniteNumber(cleaned)
	if !ok || value < 0 {
		return 0
	}
	return jscompat.Round(value*100) / 100
}

// FormatCents renders cents as US dollars: "$6,228.75", "-$5.00".
func FormatCents(cents int64) string {
	sign := ""
	abs := cents
	if cents < 0 {
		sign = "-"
		abs = -cents
	}

	dollars := strconv.FormatInt(abs/100, 10)
	var grouped strings.Builder
	for i, digit := range dollars {
		if i > 0 && (len(dollars)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(digit)
	}

	remainder := abs % 100
	pad := ""
	if remainder < 10 {
		pad = "0"
	}
	return sign + "$" + grouped.String() + "." + pad + strconv.FormatInt(remainder, 10)
}

// CentsToInput renders cents as a bare "165.00" suitable for a number input.
func CentsToInput(cents int64) string {
	return strconv.FormatFloat(float64(cents)/100, 'f', 2, 64)
}

// FormatHours renders hours with two decimals: "7.50".
func FormatHours(hours float64) string {
	return strconv.FormatFloat(hours, 'f', 2, 64)
}

// Line is the part of a line item that matters for totals.
type Line struct {
	Hours     float64
	RateCents int64
}

type Totals struct {
	Hours         float64
	SubtotalCents int64
	ExpensesCents int64
	DiscountCents int64
	TotalCents    int64
}

func ComputeTotals(lines []Line, expensesCents, discountCents int64) Totals {
	var hours float64
	var subtotal int64

	for _, line := range lines {
		hours += line.Hours
		subtotal += int64(jscompat.Round(line.Hours * float64(line.RateCents)))
	}

	return Totals{
		Hours:         jscompat.Round(hours*100) / 100,
		SubtotalCents: subtotal,
		ExpensesCents: expensesCents,
		DiscountCents: discountCents,
		TotalCents:    subtotal + expensesCents - discountCents,
	}
}
