package store

import "time"

// User is a registered account (table users).
type User struct {
	ID           string    `gorm:"type:uuid;primaryKey"`
	Email        string    `gorm:"not null;uniqueIndex:users_email_key;check:users_email_check,email = lower(email)"`
	PasswordHash string    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null;default:now();autoCreateTime:false"`
}

// TableName implements gorm.Tabler.
func (User) TableName() string { return "users" }

// RefreshToken is a stored (hashed) refresh token (table refresh_tokens).
type RefreshToken struct {
	ID         string     `gorm:"type:uuid;primaryKey"`
	UserID     string     `gorm:"type:uuid;not null;index:idx_refresh_tokens_user_active,where:revoked_at IS NULL"`
	TokenHash  string     `gorm:"not null;uniqueIndex:refresh_tokens_token_hash_key"` // sha256 hex of the opaque token
	ExpiresAt  time.Time  `gorm:"not null"`
	RevokedAt  *time.Time ``
	ReplacedBy *string    `gorm:"type:uuid"`
	CreatedAt  time.Time  `gorm:"not null;default:now();autoCreateTime:false"`

	// User only declares the foreign key (ON DELETE CASCADE); it is never loaded or saved.
	User *User `gorm:"foreignKey:UserID;references:ID;constraint:refresh_tokens_user_id_fkey,OnDelete:CASCADE"`
}

// TableName implements gorm.Tabler.
func (RefreshToken) TableName() string { return "refresh_tokens" }

// Models returns every identity model, for database.Migrate.
func Models() []any { return []any{&User{}, &RefreshToken{}} }
