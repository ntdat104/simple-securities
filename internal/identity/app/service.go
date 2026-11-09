// Package app implements the identity use cases: register, login, refresh rotation and logout.
package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"simple-securities/internal/identity/domain"
	"simple-securities/internal/identity/store"
	"simple-securities/pkg/apperr"
	"simple-securities/pkg/auth"
	"simple-securities/pkg/database"
	"simple-securities/pkg/events"
	"simple-securities/pkg/outbox"
)

// BcryptCost is the work factor for password hashes.
const BcryptCost = bcrypt.DefaultCost + 2

const source = "identity"

// Session is the result of a successful authentication.
type Session struct {
	User         store.User
	AccessToken  string
	RefreshToken string
	AccessTTL    time.Duration
}

// Service implements the identity use cases.
type Service struct {
	db         *gorm.DB
	jwt        *auth.JWT
	dispatcher *outbox.Dispatcher
	refreshTTL time.Duration
	dummyHash  []byte
	log        *zap.Logger
}

// NewService builds the service. It pre-computes a dummy bcrypt hash so logins for unknown users
// take about as long as logins for existing ones (no user enumeration through timing).
func NewService(db *gorm.DB, jwt *auth.JWT, d *outbox.Dispatcher, refreshTTL time.Duration,
	log *zap.Logger,
) (*Service, error) {
	dummy, err := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("dummy hash: %w", err)
	}
	return &Service{db: db, jwt: jwt, dispatcher: d, refreshTTL: refreshTTL, dummyHash: dummy, log: log}, nil
}

// Register creates a user and its first session, and emits user.registered.
func (s *Service) Register(ctx context.Context, email, password string) (Session, error) {
	email = domain.NormalizeEmail(email)
	if err := domain.ValidateEmail(email); err != nil {
		return Session{}, apperr.Invalid("INVALID_EMAIL", "invalid email address")
	}
	if err := domain.ValidatePassword(password); err != nil {
		return Session{}, apperr.Invalid("INVALID_PASSWORD", "%s", err.Error())
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return Session{}, fmt.Errorf("hash password: %w", err)
	}
	user := store.User{ID: uuid.Must(uuid.NewV7()).String(), Email: email, PasswordHash: string(hash)}

	var refresh string
	err = database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		created, err := store.InsertUser(ctx, tx, user)
		if err != nil {
			return err
		}
		user.CreatedAt = created
		if refresh, err = s.issueRefresh(ctx, tx, user.ID); err != nil {
			return err
		}
		env, err := events.New(source, events.TypeUserRegistered, user.ID,
			events.UserRegistered{UserID: user.ID, Email: user.Email})
		if err != nil {
			return fmt.Errorf("build event: %w", err)
		}
		return outbox.Add(ctx, tx, events.TopicUserEvents, user.ID, env)
	})
	if errors.Is(err, store.ErrEmailTaken) {
		return Session{}, apperr.Conflict("EMAIL_TAKEN", "email is already registered")
	}
	if err != nil {
		return Session{}, err
	}
	s.dispatcher.Notify()
	return s.session(user, refresh)
}

// Login verifies credentials and opens a new session.
func (s *Service) Login(ctx context.Context, email, password string) (Session, error) {
	email = domain.NormalizeEmail(email)
	user, err := store.UserByEmail(ctx, s.db, email)
	switch {
	case errors.Is(err, store.ErrNotFound):
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password)) // equalize timing
		return Session{}, invalidCredentials()
	case err != nil:
		return Session{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return Session{}, invalidCredentials()
	}
	refresh, err := s.issueRefresh(ctx, s.db, user.ID)
	if err != nil {
		return Session{}, err
	}
	return s.session(user, refresh)
}

// errTokenExpired aborts the refresh transaction for an expired token.
var errTokenExpired = errors.New("refresh token expired")

// Refresh rotates a refresh token. Presenting an already revoked token is treated as theft:
// every session of the user is revoked.
func (s *Service) Refresh(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, invalidRefresh()
	}
	var (
		user    store.User
		refresh string
		reused  bool
	)
	err := database.WithTx(ctx, s.db, func(tx *gorm.DB) error {
		old, err := store.RefreshTokenForUpdate(ctx, tx, domain.HashToken(token))
		if err != nil {
			return err
		}
		if old.RevokedAt != nil {
			n, err := store.RevokeUserTokens(ctx, tx, old.UserID)
			if err != nil {
				return err
			}
			s.log.Warn("refresh token reuse detected, sessions revoked",
				zap.String("user_id", old.UserID), zap.Int64("revoked", n))
			reused = true
			return nil // commit the revocation
		}
		if time.Now().After(old.ExpiresAt) {
			return errTokenExpired
		}
		if user, err = store.UserByID(ctx, tx, old.UserID); err != nil {
			return err
		}
		newID := uuid.Must(uuid.NewV7()).String()
		raw, hash, err := domain.NewRefreshToken()
		if err != nil {
			return err
		}
		if err := store.InsertRefreshToken(ctx, tx, newID, user.ID, hash, time.Now().Add(s.refreshTTL)); err != nil {
			return err
		}
		refresh = raw
		return store.RotateRefreshToken(ctx, tx, old.ID, newID)
	})
	switch {
	case errors.Is(err, store.ErrNotFound), errors.Is(err, errTokenExpired):
		return Session{}, invalidRefresh()
	case err != nil:
		return Session{}, err
	case reused:
		return Session{}, apperr.Unauthenticated("REFRESH_TOKEN_REUSED", "refresh token was already used")
	}
	return s.session(user, refresh)
}

// Logout revokes the refresh token. It is idempotent and ignores unknown tokens.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return apperr.Invalid("INVALID_REFRESH_TOKEN", "refresh_token is required")
	}
	return store.RevokeRefreshTokenByHash(ctx, s.db, domain.HashToken(token))
}

// GetUser returns a user by id.
func (s *Service) GetUser(ctx context.Context, id string) (store.User, error) {
	if _, err := uuid.Parse(id); err != nil {
		return store.User{}, apperr.Invalid("INVALID_USER_ID", "invalid user id")
	}
	u, err := store.UserByID(ctx, s.db, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.User{}, apperr.NotFound("USER_NOT_FOUND", "user not found")
	}
	return u, err
}

func (s *Service) issueRefresh(ctx context.Context, db *gorm.DB, userID string) (string, error) {
	raw, hash, err := domain.NewRefreshToken()
	if err != nil {
		return "", err
	}
	id := uuid.Must(uuid.NewV7()).String()
	if err := store.InsertRefreshToken(ctx, db, id, userID, hash, time.Now().Add(s.refreshTTL)); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *Service) session(u store.User, refresh string) (Session, error) {
	access, err := s.jwt.Issue(u.ID, u.Email)
	if err != nil {
		return Session{}, fmt.Errorf("issue access token: %w", err)
	}
	return Session{User: u, AccessToken: access, RefreshToken: refresh, AccessTTL: s.jwt.TTL()}, nil
}

func invalidCredentials() error {
	return apperr.Unauthenticated("INVALID_CREDENTIALS", "invalid email or password")
}

func invalidRefresh() error {
	return apperr.Unauthenticated("INVALID_REFRESH_TOKEN", "invalid or expired refresh token")
}
