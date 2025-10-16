package repo

import (
	"context"
	"simple-securities/internal/user/domain/model"
)

type IUserRepo interface {
	FindById(ctx context.Context, id uint64) (*model.User, error)
	FindByIdIn(ctx context.Context, ids []uint64) ([]*model.User, error)
	FindByUuid(ctx context.Context, uuid string) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)

	Save(ctx context.Context, user *model.User) (*model.User, error)
	SaveAll(ctx context.Context, users []*model.User) ([]*model.User, error)
}
