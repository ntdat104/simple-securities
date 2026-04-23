package constant

import (
	"simple-securities/pkg/registry"
	"time"
)

var (
	UserProfile = registry.NewDefaultCache("user:v1:profile")
	UserCash    = registry.NewDefaultCache("user:v1:cash")
	UserHistory = registry.NewCache("user:v1:history", 2*time.Minute, 4*time.Minute)
)

// Redis key patterns
const (
	LockKeyUserCredit    = "lock:credit:user:%d"        // distributed lock per user
	LockKeyReservation   = "lock:credit:reservation:%s" // lock per reservation uuid
	IdempotencyKeyPrefix = "idem:credit:%s"             // idempotency cache key
	ReservationTTL       = 10 * 60                      // 10 minutes in seconds
)
