package booking

import (
	"fmt"
	"time"

	"desksharing/internal/auth"
)

const maxResourceDays = 366

// DailyWindow treats startDate and endDate as inclusive calendar days in Europe/Berlin.
// The stored range is [start 00:00, day after end 00:00).
func DailyWindow(startDate, endDate time.Time) (time.Time, time.Time, error) {
	loc := berlinLocation()
	start := time.Date(startDate.In(loc).Year(), startDate.In(loc).Month(), startDate.In(loc).Day(), 0, 0, 0, 0, loc)
	endDay := time.Date(endDate.In(loc).Year(), endDate.In(loc).Month(), endDate.In(loc).Day(), 0, 0, 0, 0, loc)
	if endDay.Before(start) {
		return time.Time{}, time.Time{}, auth.ErrInvalidInput("Enddatum liegt vor dem Startdatum")
	}
	end := endDay.AddDate(0, 0, 1)
	days := 0
	for t := start; t.Before(end); t = t.AddDate(0, 0, 1) {
		days++
	}
	if days < 1 || days > maxResourceDays {
		return time.Time{}, time.Time{}, auth.ErrInvalidInput(fmt.Sprintf("Buchung muss 1 bis %d Tage umfassen", maxResourceDays))
	}
	return start.UTC(), end.UTC(), nil
}

// HourlyWindow keeps a same-day range in 15-minute steps.
func HourlyWindow(start, end time.Time) (time.Time, time.Time, error) {
	if !end.After(start) {
		return time.Time{}, time.Time{}, auth.ErrInvalidInput("Ende muss nach dem Beginn liegen")
	}
	if !onQuarterHour(start) || !onQuarterHour(end) {
		return time.Time{}, time.Time{}, auth.ErrInvalidInput("Zeiten nur in 15-Minuten-Schritten (00, 15, 30, 45)")
	}
	loc := berlinLocation()
	s := start.In(loc)
	e := end.In(loc)
	if s.Year() != e.Year() || s.YearDay() != e.YearDay() {
		return time.Time{}, time.Time{}, auth.ErrInvalidInput("Stundenbuchung muss am selben Tag liegen")
	}
	return start.UTC(), end.UTC(), nil
}

// InferResourceMode reports daily when the range is aligned to Berlin midnights.
func InferResourceMode(start, end time.Time) string {
	loc := berlinLocation()
	s := start.In(loc)
	e := end.In(loc)
	if isBerlinMidnight(s) && isBerlinMidnight(e) && e.After(s) {
		return "daily"
	}
	return "hourly"
}

func isBerlinMidnight(t time.Time) bool {
	return t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0
}

func resourceDisplay(group, name string, seats *int) string {
	label := name
	if seats != nil && *seats > 0 {
		label = fmt.Sprintf("%s (%d Sitze)", name, *seats)
	}
	if group != "" {
		return group + ": " + label
	}
	return label
}
