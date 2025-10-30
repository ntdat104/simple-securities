package model

import (
	"reflect"
	"simple-securities/internal/user/application/enum"
	"simple-securities/pkg/uuid"
	"strings"
	"time"
)

type User struct {
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

func (u User) TableName() string {
	return "users"
}

func (u User) QueryFields() string {
	t := reflect.TypeOf(u)
	var fields []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		dbTag := field.Tag.Get("db")
		if dbTag != "" {
			parts := strings.Split(dbTag, ",")
			fields = append(fields, parts[0])
		}
	}
	return strings.Join(fields, ", ")
}

func NewUser(email, hashedPassword string) (*User, error) {
	if email == "" {
		return nil, ErrInvalidUserEmail
	}

	if hashedPassword == "" {
		return nil, ErrUserPasswordMissing
	}

	now := time.Now()
	return &User{
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
