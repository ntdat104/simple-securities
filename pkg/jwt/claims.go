package jwt

import "github.com/golang-jwt/jwt/v5"

type Claims struct {
	UserID   uint64 `json:"user_id"`
	UserUUID string `json:"user_uuid"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}
