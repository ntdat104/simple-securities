package repo

import (
	"context"

	"simple-securities/domain/model"
)

// IExampleRepo defines the interface for example repository
type IExampleRepo interface {
	Create(ctx context.Context, example *model.Example) (*model.Example, error)
	Delete(ctx context.Context, id int) error
	Update(ctx context.Context, entity *model.Example) error
	GetByID(ctx context.Context, Id int) (*model.Example, error)
	FindByName(ctx context.Context, name string) (*model.Example, error)
}

// IExampleCacheRepo defines the interface for example cache repository
type IExampleCacheRepo interface {
	HealthCheck(ctx context.Context) error
	GetByID(ctx context.Context, id int) (*model.Example, error)
	GetByName(ctx context.Context, name string) (*model.Example, error)
	Set(ctx context.Context, example *model.Example) error
	Delete(ctx context.Context, id int) error
	Invalidate(ctx context.Context) error
}
