package jwt

import "errors"

var (
	ErrUnexpectedSigningMethod = errors.New("unexpected signing method")
	ErrInvalidOrExpiredToken   = errors.New("invalid or expired JWT token")
)
