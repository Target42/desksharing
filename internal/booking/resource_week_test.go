package booking

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAssignResourceDaysSplitsWeek(t *testing.T) {
	loc := berlinLocation()
	week := time.Date(2026, 9, 28, 0, 0, 0, 0, loc) // Monday
	beamer := uuid.New()
	car := uuid.New()
	dailyID := uuid.New()
	hourlyID := uuid.New()

	rows := assignResourceDays(
		[]resourceWeekMeta{
			{id: beamer, name: "Beamer", locationName: "Standort"},
			{id: car, name: "PKW", locationName: "Standort"},
		},
		[]resourceWeekBooking{
			{
				id: dailyID, resourceID: beamer, userName: "Ada", status: "booked",
				startsAt: time.Date(2026, 9, 29, 0, 0, 0, 0, loc).UTC(),
				endsAt:   time.Date(2026, 10, 1, 0, 0, 0, 0, loc).UTC(),
			},
			{
				id: hourlyID, resourceID: car, userName: "Ben", status: "checked_in",
				startsAt: time.Date(2026, 9, 30, 9, 0, 0, 0, loc).UTC(),
				endsAt:   time.Date(2026, 9, 30, 12, 0, 0, 0, loc).UTC(),
			},
		},
		week,
	)
	if len(rows) != 2 {
		t.Fatalf("rows: got %d", len(rows))
	}

	beamerTue := rows[0].Days[1]
	if len(beamerTue.Bookings) != 1 || beamerTue.Bookings[0].Mode != "daily" || beamerTue.Bookings[0].UserName != "Ada" {
		t.Fatalf("tuesday daily: %+v", beamerTue.Bookings)
	}
	beamerWed := rows[0].Days[2]
	if len(beamerWed.Bookings) != 1 {
		t.Fatalf("wednesday should still be booked, got %+v", beamerWed.Bookings)
	}
	if len(rows[0].Days[0].Bookings) != 0 || len(rows[0].Days[3].Bookings) != 0 {
		t.Fatal("daily booking should not spill onto Monday or Thursday")
	}

	carWed := rows[1].Days[2]
	if len(carWed.Bookings) != 1 || carWed.Bookings[0].Mode != "hourly" || carWed.Bookings[0].ID != hourlyID {
		t.Fatalf("wednesday hourly: %+v", carWed.Bookings)
	}
	if len(rows[1].Days[1].Bookings) != 0 {
		t.Fatal("hourly booking should stay on Wednesday")
	}
}
