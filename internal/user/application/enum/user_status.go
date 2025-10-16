package enum

// UserStatus defines the status of a user account.
type UserStatus string

const (
	StatusActive   UserStatus = "active"
	StatusInactive UserStatus = "inactive"
	StatusPending  UserStatus = "pending" // e.g., waiting for email confirmation
	StatusLocked   UserStatus = "locked"
)
