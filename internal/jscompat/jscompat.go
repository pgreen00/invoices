// Package jscompat reproduces the handful of JavaScript number and string
// semantics the original Node app relied on, so that form input is parsed and
// rounded exactly the way it was before the port.
package jscompat

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// IsSpace reports whether r is whitespace as defined by ECMAScript's
// String.prototype.trim (WhiteSpace + LineTerminator). It differs from
// unicode.IsSpace in that it includes U+FEFF and excludes U+0085.
func IsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', '\u00a0', '\u1680', '\u2028', '\u2029',
		'\u202f', '\u205f', '\u3000', '\ufeff':
		return true
	}
	return r >= '\u2000' && r <= '\u200a'
}

// Trim is String.prototype.trim.
func Trim(s string) string {
	return strings.TrimFunc(s, IsSpace)
}

// Round is Math.round: halves round towards +Infinity, unlike math.Round.
func Round(x float64) float64 {
	return math.Floor(x + 0.5)
}

var decimalLiteral = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)

// FiniteNumber mirrors `Number.isFinite(Number(s))` for decimal input: it
// returns the parsed value and whether it is a finite number. An empty string
// is 0, as in JavaScript.
func FiniteNumber(s string) (float64, bool) {
	s = Trim(s)
	if s == "" {
		return 0, true
	}
	if !decimalLiteral.MatchString(s) {
		return 0, false
	}
	value, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		return 0, false
	}
	return value, true
}

// ParseInt mirrors parseInt(s, 10): leading whitespace and an optional sign,
// then as many decimal digits as are present. Trailing junk is ignored.
func ParseInt(s string) (int64, bool) {
	s = strings.TrimLeftFunc(s, IsSpace)
	end := 0
	if end < len(s) && (s[end] == '+' || s[end] == '-') {
		end++
	}
	digitsStart := end
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == digitsStart {
		return 0, false
	}
	value, err := strconv.ParseInt(s[:end], 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}
