package booking

import (
	"testing"
	"time"
)

func TestDailyWindowInclusive(t *testing.T) {
	loc := berlinLocation()
	start := time.Date(2026, 10, 1, 15, 0, 0, 0, loc)
	end := time.Date(2026, 10, 3, 8, 0, 0, 0, loc)
	gotStart, gotEnd, err := DailyWindow(start, end)
	if err != nil {
		t.Fatal(err)
	}
	if !gotStart.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, loc).UTC()) {
		t.Fatalf("start %s", gotStart)
	}
	if !gotEnd.Equal(time.Date(2026, 10, 4, 0, 0, 0, 0, loc).UTC()) {
		t.Fatalf("end %s", gotEnd)
	}
	if InferResourceMode(gotStart, gotEnd) != "daily" {
		t.Fatal("expected daily")
	}
}

func TestDailyWindowRejectsReverse(t *testing.T) {
	loc := berlinLocation()
	_, _, err := DailyWindow(time.Date(2026, 10, 3, 0, 0, 0, 0, loc), time.Date(2026, 10, 1, 0, 0, 0, 0, loc))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHourlyWindowSameDay(t *testing.T) {
	loc := berlinLocation()
	start := time.Date(2026, 10, 1, 9, 0, 0, 0, loc)
	end := time.Date(2026, 10, 1, 11, 30, 0, 0, loc)
	gotStart, gotEnd, err := HourlyWindow(start, end)
	if err != nil {
		t.Fatal(err)
	}
	if InferResourceMode(gotStart, gotEnd) != "hourly" {
		t.Fatal("expected hourly")
	}
}

func TestDailyWindowSpringForward(t *testing.T) {
	loc := berlinLocation()
	day := time.Date(2026, 3, 29, 12, 0, 0, 0, loc)
	gotStart, gotEnd, err := DailyWindow(day, day)
	if err != nil {
		t.Fatal(err)
	}
	if InferResourceMode(gotStart, gotEnd) != "daily" {
		t.Fatal("expected daily")
	}
}

func TestHourlyWindowRejectsOvernight(t *testing.T) {
	loc := berlinLocation()
	start := time.Date(2026, 10, 1, 22, 0, 0, 0, loc)
	end := time.Date(2026, 10, 2, 1, 0, 0, 0, loc)
	if _, _, err := HourlyWindow(start, end); err == nil {
		t.Fatal("expected error")
	}
}
