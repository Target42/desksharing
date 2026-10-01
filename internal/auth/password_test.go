package auth

import "testing"

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("ein-langes-passwort", "a@b.de"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePassword("kurz", "a@b.de"); err == nil {
		t.Fatal("expected short password to fail")
	}
	if err := ValidatePassword("changeme1234", "a@b.de"); err == nil {
		t.Fatal("expected denylisted password to fail")
	}
	if err := ValidatePassword("person@firma.de", "person@firma.de"); err == nil {
		t.Fatal("expected email-as-password to fail")
	}
}

func TestTimingHashIsBcrypt(t *testing.T) {
	if !CheckPassword(bcryptHashForTiming, "timing-only") {
		t.Fatal("timing hash is not a usable bcrypt hash")
	}
}
