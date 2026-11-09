// Package store holds the identity GORM models and repositories. Every function takes a *gorm.DB
// so it can run on the pool or inside a transaction.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-securities/pkg/database"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

// ErrEmailTaken is returned when a user with the same e-mail exists.
var ErrEmailTaken = errors.New("email taken")

// InsertUser inserts u and returns its creation time. A duplicate e-mail yields ErrEmailTaken.
func InsertUser(ctx context.Context, db *gorm.DB, u User) (time.Time, error) {
	u.CreatedAt = time.Time{} // let the database default (now()) apply; returned via RETURNING
	err := db.WithContext(ctx).Omit(clause.Associations).Create(&u).Error
	if database.IsUniqueViolation(err) {
		return time.Time{}, ErrEmailTaken
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("insert user: %w", err)
	}
	return u.CreatedAt, nil
}

// UserByEmail loads a user by normalized e-mail.
func UserByEmail(ctx context.Context, db *gorm.DB, email string) (User, error) {
	return findUser(db.WithContext(ctx).Where("email = ?", email))
}

// UserByID loads a user by id.
func UserByID(ctx context.Context, db *gorm.DB, id string) (User, error) {
	return findUser(db.WithContext(ctx).Where("id = ?", id))
}

func findUser(q *gorm.DB) (User, error) {
	var u User
	if err := q.Take(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("select user: %w", err)
	}
	return u, nil
}

// InsertRefreshToken stores a hashed refresh token.
func InsertRefreshToken(ctx context.Context, db *gorm.DB, id, userID, hash string, expiresAt time.Time) error {
	t := RefreshToken{ID: id, UserID: userID, TokenHash: hash, ExpiresAt: expiresAt}
	if err := db.WithContext(ctx).Omit(clause.Associations).Create(&t).Error; err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

// RefreshTokenForUpdate loads and row-locks (SELECT ... FOR UPDATE) a refresh token by hash.
func RefreshTokenForUpdate(ctx context.Context, db *gorm.DB, hash string) (RefreshToken, error) {
	var t RefreshToken
	err := db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("token_hash = ?", hash).Take(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return RefreshToken{}, ErrNotFound
	}
	if err != nil {
		return RefreshToken{}, fmt.Errorf("select refresh token: %w", err)
	}
	return t, nil
}

// RotateRefreshToken revokes token id and links it to its replacement.
func RotateRefreshToken(ctx context.Context, db *gorm.DB, id, replacedBy string) error {
	err := db.WithContext(ctx).Model(&RefreshToken{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Updates(map[string]any{"revoked_at": gorm.Expr("now()"), "replaced_by": replacedBy}).Error
	if err != nil {
		return fmt.Errorf("rotate refresh token: %w", err)
	}
	return nil
}

// RevokeRefreshTokenByHash revokes a token if it is still active. Unknown tokens are ignored.
func RevokeRefreshTokenByHash(ctx context.Context, db *gorm.DB, hash string) error {
	err := db.WithContext(ctx).Model(&RefreshToken{}).
		Where("token_hash = ? AND revoked_at IS NULL", hash).
		Update("revoked_at", gorm.Expr("now()")).Error
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

// RevokeUserTokens revokes every active refresh token of a user.
func RevokeUserTokens(ctx context.Context, db *gorm.DB, userID string) (int64, error) {
	res := db.WithContext(ctx).Model(&RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", gorm.Expr("now()"))
	if res.Error != nil {
		return 0, fmt.Errorf("revoke user tokens: %w", res.Error)
	}
	return res.RowsAffected, nil
}
