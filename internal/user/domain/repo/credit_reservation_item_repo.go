package repo

import (
	"context"

	"github.com/jmoiron/sqlx"

	"simple-securities/internal/user/domain/model"
)

type ICreditReservationItemRepo interface {
	FindItemsByReservationID(ctx context.Context, reservationID uint64) ([]*model.CreditReservationItem, error)
	SaveAll(ctx context.Context, tx *sqlx.Tx, items []*model.CreditReservationItem) ([]*model.CreditReservationItem, error)
}
