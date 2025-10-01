package repo

import (
	"context"
	"simple-securities/internal/notification/domain/model"
)

type INotificationRepo interface {
	Create(ctx context.Context, notification *model.Notification) (*model.Notification, error)
	Delete(ctx context.Context, id uint64) error
	Update(ctx context.Context, notification *model.Notification) error
	GetByID(ctx context.Context, id uint64) (*model.Notification, error)
	GetByUserId(ctx context.Context, userId uint64, limit, offset uint32) ([]*model.Notification, error)
}

type INotificationCacheRepo interface {
	Set(ctx context.Context, key uint64, notification *model.Notification) error
	Get(ctx context.Context, id uint64) (*model.Notification, error)
	Invalidate(ctx context.Context, id uint64) error
	InvalidateByUserId(ctx context.Context, userId uint64) error
	InvalidateAll(ctx context.Context) error
}
