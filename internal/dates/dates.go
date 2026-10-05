// Package dates handles calendar dates stored as plain ISO "YYYY-MM-DD"
// strings. They are always treated as calendar dates, never instants, so no
// timezone can shift them by a day. Arithmetic is done on UTC midnights purely
// as a calendar; the zone never leaks into the result.
package dates

import (
	"regexp"
	"strconv"
	"time"
)

const isoLayout = "2006-01-02"

var isoPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

var months = [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
var days = [...]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

func IsISO(value string) bool {
	return isoPattern.MatchString(value)
}

// Parse reads an ISO date. Out-of-range parts roll over the way JavaScript's
// Date constructor does ("2024-02-30" is March 1st).
func Parse(iso string) (time.Time, bool) {
	if !IsISO(iso) {
		return time.Time{}, false
	}
	y, _ := strconv.Atoi(iso[0:4])
	m, _ := strconv.Atoi(iso[5:7])
	d, _ := strconv.Atoi(iso[8:10])
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC), true
}

func ToISO(date time.Time) string {
	return date.Format(isoLayout)
}

// Today is the current local calendar date.
func Today() string {
	return time.Now().Format(isoLayout)
}

func AddDays(iso string, n int) string {
	date, ok := Parse(iso)
	if !ok {
		return iso
	}
	return ToISO(date.AddDate(0, 0, n))
}

// MondayOfWeek returns the Monday of the week containing iso (weeks run
// Monday–Sunday).
func MondayOfWeek(iso string) string {
	date, ok := Parse(iso)
	if !ok {
		return iso
	}
	dow := int(date.Weekday()) // 0 = Sunday
	offset := 1 - dow
	if dow == 0 {
		offset = -6
	}
	return ToISO(date.AddDate(0, 0, offset))
}

// WorkWeekOf returns the Mon–Fri window containing iso.
func WorkWeekOf(iso string) (start, end string) {
	start = MondayOfWeek(iso)
	return start, AddDays(start, 4)
}

// WeekdaysFrom returns five ISO dates, Monday through Friday, of the week
// containing startISO.
func WeekdaysFrom(startISO string) []string {
	monday := MondayOfWeek(startISO)
	out := make([]string, 5)
	for i := range out {
		out[i] = AddDays(monday, i)
	}
	return out
}

func monthDay(date time.Time) string {
	return months[date.Month()-1] + " " + strconv.Itoa(date.Day())
}

// FormatLong renders "Mar 24, 2024".
func FormatLong(iso string) string {
	date, ok := Parse(iso)
	if !ok {
		return ""
	}
	return monthDay(date) + ", " + strconv.Itoa(date.Year())
}

// FormatDayDate renders "Mon, Mar 18".
func FormatDayDate(iso string) string {
	date, ok := Parse(iso)
	if !ok {
		return ""
	}
	return days[date.Weekday()] + ", " + monthDay(date)
}

// FormatRange renders "Mar 18 – 22, 2024" or "Mar 28 – Apr 1, 2024".
func FormatRange(startISO, endISO string) string {
	start, startOK := Parse(startISO)
	end, endOK := Parse(endISO)
	switch {
	case !startOK && !endOK:
		return ""
	case !startOK:
		return FormatLong(endISO)
	case !endOK:
		return FormatLong(startISO)
	}

	sameYear := start.Year() == end.Year()
	sameMonth := sameYear && start.Month() == end.Month()

	left := monthDay(start)
	right := monthDay(end)
	if sameMonth {
		right = strconv.Itoa(end.Day())
	}

	if sameYear {
		return left + " – " + right + ", " + strconv.Itoa(end.Year())
	}
	return left + ", " + strconv.Itoa(start.Year()) + " – " + right + ", " + strconv.Itoa(end.Year())
}

// Timestamp is the format generated_at/updated_at are stored in, identical to
// JavaScript's Date.prototype.toISOString.
const Timestamp = "2006-01-02T15:04:05.000Z"

// Now returns the current instant formatted as a stored timestamp.
func Now() string {
	return time.Now().UTC().Format(Timestamp)
}

// FormatTimestamp renders a stored timestamp in local time:
// "Mar 24, 2024 at 2:14 PM".
func FormatTimestamp(value string) string {
	if value == "" {
		return ""
	}
	instant, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return ""
	}
	local := instant.Local()
	return monthDay(local) + ", " + strconv.Itoa(local.Year()) + " at " + local.Format("3:04 PM")
}
