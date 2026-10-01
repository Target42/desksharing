package auth

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeTeamName(t *testing.T) {
	name, err := normalizeTeamName("  Vertrieb  ")
	if err != nil || name != "Vertrieb" {
		t.Fatalf("got %q %v", name, err)
	}
	if _, err := normalizeTeamName("   "); err == nil {
		t.Fatal("empty name accepted")
	}
	if _, err := normalizeTeamName(strings.Repeat("ä", 81)); err == nil {
		t.Fatal("long name accepted")
	}
	if _, err := normalizeTeamName(strings.Repeat("ä", 80)); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeMemberIDs(t *testing.T) {
	id := uuid.New()
	other := uuid.New()
	got, err := normalizeMemberIDs([]uuid.UUID{id, id, other})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != id || got[1] != other {
		t.Fatalf("got %v", got)
	}
	if _, err := normalizeMemberIDs([]uuid.UUID{uuid.Nil}); err == nil {
		t.Fatal("nil id accepted")
	}
	many := make([]uuid.UUID, maxTeamMembers+1)
	for i := range many {
		many[i] = uuid.New()
	}
	if _, err := normalizeMemberIDs(many); err == nil {
		t.Fatal("too many members accepted")
	}
}
