package enum

type UserStatus string

const (
	UserActive     UserStatus = "ACTIVE"
	UserInactive   UserStatus = "INACTIVE"
	UserProcessing UserStatus = "PROCESSING"
	UserLocked     UserStatus = "LOCKED"
)
