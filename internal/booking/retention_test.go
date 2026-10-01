package booking

import (
	"strings"
	"testing"
)

func TestOccupancySQLStoresDaysWithoutIdentity(t *testing.T) {
	sql := occupancyInsertSQL("place_occupancy_days", "place_id", "bk.place_id")
	if strings.Contains(sql, "email") || strings.Contains(sql, "user_id") || strings.Contains(sql, "display_name") {
		t.Fatalf("occupancy must not copy who booked:\n%s", sql)
	}
	if !strings.Contains(sql, "place_occupancy_days") || !strings.Contains(sql, "ON CONFLICT") {
		t.Fatal(sql)
	}
}
