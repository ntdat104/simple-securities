package repo

import (
	"context"
	"simple-securities/internal/user/domain/model"

	"github.com/jmoiron/sqlx"
)

type IUserHistoryRepo interface {
	FindAllByPageAndSize(ctx context.Context, page int, size int) ([]*model.UserHistory, error)

	FindById(ctx context.Context, id uint64) (*model.UserHistory, error)
	FindByIdIn(ctx context.Context, ids []uint64) ([]*model.UserHistory, error)

	FindByIdAndEmailAndUuid(ctx context.Context, id uint64, email string, uuid string) (*model.UserHistory, error)
	FindByEmail(ctx context.Context, email string) (*model.UserHistory, error)

	FindByUuid(ctx context.Context, uuid string) (*model.UserHistory, error)
	FindByUuidIn(ctx context.Context, uuids []string) ([]*model.UserHistory, error)

	Save(ctx context.Context, tx *sqlx.Tx, user *model.UserHistory) (*model.UserHistory, error)
	SaveAll(ctx context.Context, tx *sqlx.Tx, users []*model.UserHistory) ([]*model.UserHistory, error)

	CountTotal(ctx context.Context) (int64, error)
}
