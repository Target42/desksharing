package auth

import "strings"

const (
	MinPasswordLen = 12
	MaxPasswordLen = 72 // bcrypt ignores bytes beyond this
)

var weakPasswords = map[string]struct{}{
	"password1234":  {},
	"passwort1234":  {},
	"changeme1234":  {},
	"changeme123":   {},
	"desksharing1":  {},
	"desksharing":   {},
	"administrator": {},
	"qwertyuiopas":  {},
	"welcome12345":  {},
	"letmein12345":  {},
	"admin1234567":  {},
}

// ValidatePassword checks length and a short denylist of obvious passwords.
func ValidatePassword(password, email string) error {
	if len(password) < MinPasswordLen {
		return ErrInvalidInput("Passwort muss mindestens 12 Zeichen haben")
	}
	if len(password) > MaxPasswordLen {
		return ErrInvalidInput("Passwort darf höchstens 72 Byte lang sein")
	}
	if _, ok := weakPasswords[strings.ToLower(password)]; ok {
		return ErrInvalidInput("Dieses Passwort ist zu üblich")
	}
	if email != "" && strings.EqualFold(strings.TrimSpace(password), strings.TrimSpace(email)) {
		return ErrInvalidInput("Passwort darf nicht die E-Mail-Adresse sein")
	}
	return nil
}
