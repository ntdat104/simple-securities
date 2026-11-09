// Package auth issues and verifies access tokens (JWT, HS256).
// Production note: switch to EdDSA/RS256 + JWKS so verifiers never hold the signing key.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims carried in the access token.
type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// JWT signs and verifies tokens.
type JWT struct {
	secret []byte
	ttl    time.Duration
	issuer string
}

// New returns a JWT helper.
func New(secret string, ttl time.Duration) *JWT {
	return &JWT{secret: []byte(secret), ttl: ttl, issuer: "simple-securities"}
}

// TTL is the access token lifetime.
func (j *JWT) TTL() time.Duration { return j.ttl }

// Issue signs an access token for the user.
func (j *JWT) Issue(userID, email string) (string, error) {
	now := time.Now()
	c := Claims{Email: email, RegisteredClaims: jwt.RegisteredClaims{
		Subject: userID, Issuer: j.issuer, IssuedAt: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(j.secret)
}

// Verify validates the token and returns its claims.
func (j *JWT) Verify(token string) (*Claims, error) {
	var c Claims
	t, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return j.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(j.issuer), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	if !t.Valid || c.Subject == "" {
		return nil, errors.New("invalid token")
	}
	return &c, nil
}
