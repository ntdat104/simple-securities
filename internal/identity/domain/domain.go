// Package domain contains the pure identity rules: credential validation and refresh tokens.
package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
)

// Password length bounds. bcrypt ignores everything after 72 bytes, so longer input is rejected.
const (
	MinPasswordLen = 8
	MaxPasswordLen = 72
	maxEmailLen    = 254
	refreshTokenSz = 32
)

var (
	// ErrInvalidEmail is returned for malformed e-mail addresses.
	ErrInvalidEmail = errors.New("invalid email address")
	// ErrInvalidPassword is returned when the password length is out of bounds.
	ErrInvalidPassword = fmt.Errorf("password must be %d..%d bytes", MinPasswordLen, MaxPasswordLen)
)

// NormalizeEmail trims and lower-cases an e-mail address.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmail checks a normalized e-mail address (bare address, domain with a dot).
func ValidateEmail(email string) error {
	if email == "" || len(email) > maxEmailLen {
		return ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || addr.Name != "" {
		return ErrInvalidEmail
	}
	at := strings.LastIndexByte(email, '@')
	if at <= 0 {
		return ErrInvalidEmail
	}
	domain := email[at+1:]
	if !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return ErrInvalidEmail
	}
	return nil
}

// ValidatePassword checks the password length in bytes.
func ValidatePassword(password string) error {
	if n := len(password); n < MinPasswordLen || n > MaxPasswordLen {
		return ErrInvalidPassword
	}
	return nil
}

// NewRefreshToken returns an opaque token (32 random bytes, base64url) and its storage hash.
func NewRefreshToken() (token, hash string, err error) {
	b := make([]byte, refreshTokenSz)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("random token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken returns the sha256 hex digest stored instead of the raw refresh token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
