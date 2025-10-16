package dto

import (
	"simple-securities/internal/user/application/enum"
	"time"
)

type UserDto struct {
	ID          uint64          `json:"id"`
	Uuid        string          `json:"uuid"`
	Email       string          `json:"email"`
	LastLoginAt *time.Time      `json:"last_login_at"`
	Status      enum.UserStatus `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// UserRegister
type UserRegisterReq struct {
	Email    string `json:"email" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type UserRegisterResp struct {
	User        *UserDto `json:"user"`
	AccessToken string   `json:"access_token"`
	TokenType   string   `json:"token_type"`
	Exp         int64    `json:"exp"`
}

// UserLogin
type UserLoginReq struct {
	Email    string `json:"email" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type UserLoginResp struct {
	User        *UserDto `json:"user"`
	AccessToken string   `json:"access_token"`
	TokenType   string   `json:"token_type"`
	Exp         int64    `json:"exp"`
}
