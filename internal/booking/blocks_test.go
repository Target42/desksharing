package booking

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPrepareBlockTargets(t *testing.T) {
	place := uuid.New()
	room := uuid.New()

	places, gotRoom, err := prepareBlockTargets([]uuid.UUID{place, place}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(places) != 1 || places[0] != place || gotRoom != nil {
		t.Fatalf("places=%v room=%v", places, gotRoom)
	}

	places, gotRoom, err = prepareBlockTargets(nil, &room)
	if err != nil {
		t.Fatal(err)
	}
	if len(places) != 0 || gotRoom == nil || *gotRoom != room {
		t.Fatalf("places=%v room=%v", places, gotRoom)
	}

	if _, _, err := prepareBlockTargets([]uuid.UUID{place}, &room); err == nil {
		t.Fatal("expected xor error")
	}
	if _, _, err := prepareBlockTargets(nil, nil); err == nil {
		t.Fatal("expected missing target")
	}
	nilRoom := uuid.Nil
	if _, _, err := prepareBlockTargets(nil, &nilRoom); err == nil {
		t.Fatal("expected missing target for nil room")
	}

	many := make([]uuid.UUID, maxBlockPlaces+1)
	for i := range many {
		many[i] = uuid.New()
	}
	if _, _, err := prepareBlockTargets(many, nil); err == nil {
		t.Fatal("expected too many places")
	}
}

func TestNormalizeBlockReason(t *testing.T) {
	got, err := normalizeBlockReason("  Umbau  ")
	if err != nil || got != "Umbau" {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err := normalizeBlockReason(strings.Repeat("ä", maxBlockReason+1)); err == nil {
		t.Fatal("expected long reason to fail")
	}
}

func TestBlockWindow(t *testing.T) {
	loc := BerlinLocation()
	start := time.Date(2026, 10, 5, 8, 0, 0, 0, loc)
	end := time.Date(2026, 10, 9, 18, 0, 0, 0, loc)
	gotStart, gotEnd, err := BlockWindow(start, end)
	if err != nil {
		t.Fatal(err)
	}
	if !gotStart.Equal(start.UTC()) || !gotEnd.Equal(end.UTC()) {
		t.Fatalf("window %s – %s", gotStart, gotEnd)
	}

	if _, _, err := BlockWindow(end, start); err == nil {
		t.Fatal("expected reverse range to fail")
	}
	off := time.Date(2026, 10, 5, 8, 10, 0, 0, loc)
	if _, _, err := BlockWindow(off, end); err == nil {
		t.Fatal("expected quarter-hour check")
	}
	if _, _, err := BlockWindow(start, start.Add(maxBlockSpan+time.Hour)); err == nil {
		t.Fatal("expected span limit")
	}
}
