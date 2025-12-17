package util

import (
	"fmt"
	"simple-securities/pkg/datetime"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateJwtToken(
	userId uint64,
	userUuid,
	email string,
	jwtSecret string,
	expire uint32,
) (string, int64, error) {
	now := datetime.Now()
	expireTime := now.Add(time.Duration(expire) * time.Second)
	expireUnix := expireTime.Unix()

	claims := jwt.MapClaims{
		"user_id":   userId,
		"user_uuid": userUuid,
		"email":     email,
		"exp":       expireUnix, // expires in 24h
		"iat":       now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	accessToken, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", 0, fmt.Errorf("Failed to generate token: %v", err)
	}

	return accessToken, expireUnix, nil
}
