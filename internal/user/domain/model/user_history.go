package model

import (
	"simple-securities/internal/user/application/constant"
	"simple-securities/internal/user/domain/enum"
	"simple-securities/pkg/datetime"
	"simple-securities/pkg/uuid"
	"time"
)

type UserHistory struct {
	ID             uint64          `db:"id"`
	Uuid           string          `db:"uuid"`
	Email          string          `db:"email"`
	HashedPassword string          `db:"hashed_password"` // Store hash, not plaintext password
	RefreshToken   string          `db:"refresh_token"`   // For session management
	LastLoginAt    *time.Time      `db:"last_login_at"`
	Status         enum.UserStatus `db:"status"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
	CreatedBy      uint64          `db:"created_by"` // Can be 0 for self-registration or an admin ID
	UpdatedBy      uint64          `db:"updated_by"`
}

func (u UserHistory) TableName() string {
	return "user_history"
}

func NewUserHistory(
	email string,
	hashedPassword string,
) (*UserHistory, error) {
	if hashedPassword == "" {
		return nil, constant.ErrUserPasswordMissing
	}

	now := datetime.Now()
	return &UserHistory{
		Uuid:           uuid.NewGoogleUUID(),
		Email:          email,
		HashedPassword: hashedPassword,
		Status:         enum.UserProcessing, // Default to pending until confirmed
		LastLoginAt:    nil,
		CreatedAt:      now,
		UpdatedAt:      now,
		CreatedBy:      0,
		UpdatedBy:      0,
	}, nil
}
