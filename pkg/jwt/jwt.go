package jwt

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
		"exp":       expireUnix,
		"iat":       now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	jwtToken, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", 0, fmt.Errorf("Failed to generate token: %v", err)
	}

	return jwtToken, expireUnix, nil
}

func ParseWithClaims(tokenString, secret string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		// Ensure signing method is HMAC (HS256)
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrUnexpectedSigningMethod
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, ErrInvalidOrExpiredToken
	}

	return claims, nil
}
