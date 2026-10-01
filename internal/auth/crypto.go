package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	PurposeTenantConfirm = "tenant_confirm"
	PurposeSession       = "session"
	PurposeInvite        = "invite"
)

// bcryptHashForTiming is a real cost-10 hash. Login compares against it when
// no stored hash exists, so missing accounts take about as long as real ones.
const bcryptHashForTiming = "$2a$10$LeAsL3IxkB5.jAo7k49SHurA94rvKEEwJC7OtpsPnRBi693moh6yy"

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// NewToken returns a URL-safe raw token and its SHA-256 hex hash for storage.
func NewToken() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return raw, hash, nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
