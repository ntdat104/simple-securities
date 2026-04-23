package repo

import (
	"context"

	"github.com/jmoiron/sqlx"

	"simple-securities/internal/user/domain/model"
)

// ICreditReservationRepo manages reservation lifecycle.
type ICreditReservationRepo interface {
	FindByUuid(ctx context.Context, uuid string) (*model.CreditReservation, error)
	FindByIdempotencyKey(ctx context.Context, key string) (*model.CreditReservation, error)

	Save(ctx context.Context, tx *sqlx.Tx, reservation *model.CreditReservation) (*model.CreditReservation, error)

	// FindExpiredPending returns PENDING reservations past their expires_at (for the sweep job).
	FindExpiredPending(ctx context.Context, limit int) ([]*model.CreditReservation, error)
}
