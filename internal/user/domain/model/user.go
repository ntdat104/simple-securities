package model

import (
	"simple-securities/internal/user/application/enum"
	"simple-securities/pkg/uuid"
	"time"
)

type User struct {
	ID             uint64          `db:"id"`
	Uuid           string          `db:"uuid"`
	Email          string          `db:"email"`
	HashedPassword string          `db:"hashed_password"` // Store hash, not plaintext password
	LastLoginAt    *time.Time      `db:"last_login_at"`
	Status         enum.UserStatus `db:"status"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
	CreatedBy      uint64          `db:"created_by"` // Can be 0 for self-registration or an admin ID
	UpdatedBy      uint64          `db:"updated_by"`
}

func (u User) TableName() string {
	return "users"
}

func NewUser(email, hashedPassword string) *User {
	now := time.Now()
	return &User{
		Uuid:           uuid.NewGoogleUUID(),
		Email:          email,
		HashedPassword: hashedPassword,
		Status:         enum.StatusPending, // Default to pending until confirmed
		LastLoginAt:    nil,
		CreatedAt:      now,
		UpdatedAt:      now,
		CreatedBy:      0,
		UpdatedBy:      0,
	}
}
