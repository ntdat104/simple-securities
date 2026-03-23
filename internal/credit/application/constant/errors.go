package constant

import "simple-securities/pkg/errors"

var (
	ErrInsufficientCredits   = errors.New(errors.ErrorTypeBusiness, "insufficient credits")
	ErrWalletNotFound        = errors.New(errors.ErrorTypeNotFound, "credit wallet not found")
	ErrReservationNotFound   = errors.New(errors.ErrorTypeNotFound, "reservation not found")
	ErrReservationExpired    = errors.New(errors.ErrorTypeBusiness, "reservation has expired")
	ErrReservationNotPending = errors.New(errors.ErrorTypeBusiness, "reservation is not in PENDING status")
	ErrDuplicateIdempotency  = errors.New(errors.ErrorTypeBusiness, "duplicate request: idempotency key already used")
	ErrInvalidAmount         = errors.New(errors.ErrorTypeBusiness, "amount must be greater than zero")
	ErrExpireAtRequired      = errors.New(errors.ErrorTypeBusiness, "expire_at is required for this wallet type")
)

// Redis key patterns
const (
	LockKeyUserCredit      = "lock:credit:user:%d"          // distributed lock per user
	LockKeyReservation     = "lock:credit:reservation:%s"   // lock per reservation uuid
	IdempotencyKeyPrefix   = "idem:credit:%s"               // idempotency cache key
	ReservationTTL         = 10 * 60                        // 10 minutes in seconds
)
