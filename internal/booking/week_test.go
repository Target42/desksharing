package booking

import (
	"testing"
	"time"
)

func TestOnQuarterHour(t *testing.T) {
	loc := berlinLocation()
	ok := time.Date(2026, 10, 2, 14, 15, 0, 0, loc)
	if !onQuarterHour(ok) {
		t.Fatal("14:15 should be allowed")
	}
	off := time.Date(2026, 10, 2, 14, 10, 0, 0, loc)
	if onQuarterHour(off) {
		t.Fatal("14:10 should be rejected")
	}
}

func TestStatusForDayUsesCoveredHours(t *testing.T) {
	loc := berlinLocation()
	day := time.Date(2026, 9, 29, 0, 0, 0, 0, loc)

	partial := []timeInterval{{
		start: time.Date(2026, 9, 29, 14, 0, 0, 0, loc),
		end:   time.Date(2026, 9, 29, 16, 0, 0, 0, loc),
		mode:  "daily",
	}}
	if got := statusForDay(day, partial, true); got != DayPartial {
		t.Fatalf("afternoon booking: got %s, want partial", got)
	}

	full := []timeInterval{{
		start: time.Date(2026, 9, 29, 8, 0, 0, 0, loc),
		end:   time.Date(2026, 9, 29, 18, 0, 0, 0, loc),
		mode:  "hourly",
	}}
	if got := statusForDay(day, full, true); got != DayFull {
		t.Fatalf("whole business day: got %s, want full", got)
	}

	if got := statusForDay(day, nil, true); got != DayFree {
		t.Fatalf("empty day: got %s, want free", got)
	}

	blocked := []timeInterval{{
		start: time.Date(2026, 9, 29, 8, 0, 0, 0, loc),
		end:   time.Date(2026, 9, 29, 12, 0, 0, 0, loc),
		mode:  "block",
	}}
	if got := statusForDay(day, blocked, true); got != DayBlocked {
		t.Fatalf("block only: got %s, want blocked", got)
	}
}
