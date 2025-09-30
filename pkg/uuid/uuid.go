package uuid

import (
	"github.com/google/uuid"
	"github.com/lithammer/shortuuid/v4"
)

// NewShortUUID generates a new short UUID string.
func NewShortUUID() string {
	return shortuuid.New()
}

// NewGoogleUUID generates a new UUID using the google/uuid package.
func NewGoogleUUID() string {
	return uuid.New().String()
}
