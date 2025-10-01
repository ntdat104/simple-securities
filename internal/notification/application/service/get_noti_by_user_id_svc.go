package service

import (
	"context"
	"simple-securities/internal/notification/application/dto"
	"simple-securities/internal/notification/application/mapper"
	"simple-securities/internal/notification/domain/repo"
)

type GetNotiByUserIdSvc interface {
	Handle(ctx context.Context, userId uint64, limit, offset uint32) ([]*dto.NotificationDto, error)
}

type getNotiByUserIdSvc struct {
	notiRepo      repo.INotificationRepo
	notiCacheRepo repo.INotificationCacheRepo
}

func NewGetNotiByUserIdSvc(
	notiRepo repo.INotificationRepo,
	notiCacheRepo repo.INotificationCacheRepo,
) GetNotiByUserIdSvc {
	return &getNotiByUserIdSvc{
		notiRepo:      notiRepo,
		notiCacheRepo: notiCacheRepo,
	}
}

func (s *getNotiByUserIdSvc) Handle(
	ctx context.Context,
	userId uint64,
	limit uint32,
	offset uint32,
) ([]*dto.NotificationDto, error) {
	notifications, err := s.notiRepo.GetByUserId(ctx, userId, limit, offset)
	if err != nil {
		return nil, err
	}
	return mapper.ToNotificationDtos(notifications), nil
}
