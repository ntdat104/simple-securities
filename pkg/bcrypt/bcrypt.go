// Package bcrypt provides a small, reusable wrapper around golang.org/x/crypto/bcrypt
// with conveniences for hashing, verifying and migrating passwords.
//
// Features:
//   - Hashing with default or custom cost
//   - Constant-time comparison via bcrypt.CompareHashAndPassword
//   - Inspecting cost from existing hash
//   - Helper to check if password needs rehash (cost change)
//   - Small, well-documented API that's easy to reuse
package bcrypt

import (
	"errors"
	"strings"

	bcr "golang.org/x/crypto/bcrypt"
)

// DefaultCost is the cost used when none is specified. Matches bcrypt.DefaultCost.
const DefaultCost = bcr.DefaultCost

// ErrInvalidHash indicates the provided hash is not a valid bcrypt hash.
var ErrInvalidHash = errors.New("invalid bcrypt hash")

// HashPassword creates a bcrypt hash from the given password string using the default cost.
// It returns the hash as a base64-able string (the standard bcrypt output).
func HashPassword(password string) (string, error) {
	return HashPasswordWithCost(password, DefaultCost)
}

// HashPasswordWithCost creates a bcrypt hash using a specific cost.
// Cost must be between MinCost and MaxCost as defined by golang.org/x/crypto/bcrypt.
func HashPasswordWithCost(password string, cost int) (string, error) {
	if password == "" {
		// allow hashing of empty password but still call the underlying function
	}
	hash, err := bcr.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// ComparePassword compares a bcrypt hash with the provided password.
// Returns nil on success, or an error from bcrypt (like bcrypt.ErrMismatchedHashAndPassword).
func ComparePassword(hash string, password string) error {
	if !IsBcryptHash(hash) {
		return ErrInvalidHash
	}
	return bcr.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GetCost extracts the cost used to create the bcrypt hash.
// Returns an error if the provided string is not a valid bcrypt hash.
func GetCost(hash string) (int, error) {
	if !IsBcryptHash(hash) {
		return 0, ErrInvalidHash
	}
	return bcr.Cost([]byte(hash))
}

// NeedsRehash reports whether the provided hash was generated with a cost different
// from the provided targetCost. If the hash is invalid, an error is returned.
func NeedsRehash(hash string, targetCost int) (bool, error) {
	c, err := GetCost(hash)
	if err != nil {
		return false, err
	}
	return c != targetCost, nil
}

// CheckAndRehash verifies the password against the hash and if the hash cost differs
// from targetCost it returns a newly generated hash and rehashed=true. If verification
// fails, the returned error is the verification error and newHash is empty.
// Example use: when the system's cost increases, run this after successful login to
// transparently upgrade stored hashes.
func CheckAndRehash(hash string, password string, targetCost int) (newHash string, rehashed bool, err error) {
	// verify first
	err = ComparePassword(hash, password)
	if err != nil {
		return "", false, err
	}
	need, err := NeedsRehash(hash, targetCost)
	if err != nil {
		return "", false, err
	}
	if !need {
		return "", false, nil
	}
	// generate new one
	nh, err := HashPasswordWithCost(password, targetCost)
	if err != nil {
		return "", false, err
	}
	return nh, true, nil
}

// IsBcryptHash performs a lightweight check to see if the string looks like a bcrypt hash.
// It checks prefix and length heuristics ("$2a$" | "$2b$" | "$2y$" etc.).
func IsBcryptHash(s string) bool {
	if s == "" {
		return false
	}
	// bcrypt hashes start with $2a$, $2b$, $2y$, etc. and include $ separators
	if !strings.HasPrefix(s, "$2") {
		return false
	}
	// minimal length check: $2a$ + 2-digit cost + 53 chars salt+hash -> usually 60
	// but allow small variance for future markers
	if len(s) < 50 || len(s) > 100 {
		// quick reject if obviously wrong length
		return false
	}
	// Defer final validation to Cost() which will error if malformed.
	return true
}

// MustHash is like HashPassword but panics on error. Useful in tests or init-time
// setup where you prefer a panic over error handling.
func MustHash(password string) string {
	h, err := HashPassword(password)
	if err != nil {
		panic(err)
	}
	return h
}

// CompareSafe returns a boolean rather than an error to signal match status.
// This simply wraps ComparePassword and converts the error into true/false.
func CompareSafe(hash string, password string) bool {
	err := ComparePassword(hash, password)
	return err == nil
}

// Exported errors from bcrypt for callers who want to compare specifically.
var (
	ErrMismatchedHashAndPassword = bcr.ErrMismatchedHashAndPassword
	ErrHashTooShort              = bcr.ErrHashTooShort
)

// Example usage (for docs):
//
//  h, err := bcrypt.HashPassword("secret")
//  if err != nil { ... }
//  if err := bcrypt.ComparePassword(h, "secret"); err != nil { ... }
//
//  // migrate to higher cost on successful login
//  if newHash, rehashed, err := bcrypt.CheckAndRehash(h, "secret", bcrypt.DefaultCost+2); err == nil && rehashed {
//      // store newHash
//  }
