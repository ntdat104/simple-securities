package repo

import (
	"context"
	"simple-securities/internal/user/domain/model"

	"github.com/jmoiron/sqlx"
)

type IUserRepo interface {
	FindAllByPageAndSize(ctx context.Context, page int, size int) ([]*model.User, error)

	FindById(ctx context.Context, id uint64) (*model.User, error)
	FindByIdIn(ctx context.Context, ids []uint64) ([]*model.User, error)

	FindByIdAndEmailAndUuid(ctx context.Context, id uint64, email string, uuid string) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)

	FindByUuid(ctx context.Context, uuid string) (*model.User, error)
	FindByUuidIn(ctx context.Context, uuids []string) ([]*model.User, error)

	Save(ctx context.Context, tx *sqlx.Tx, user *model.User) (*model.User, error)
	SaveAll(ctx context.Context, tx *sqlx.Tx, users []*model.User) ([]*model.User, error)

	CountTotal(ctx context.Context) (int64, error)
}
