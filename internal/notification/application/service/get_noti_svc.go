package service

import (
	"context"
	"simple-securities/internal/notification/application/dto"
	"simple-securities/internal/notification/application/mapper"
	"simple-securities/internal/notification/domain/repo"
)

type GetNotiSvc interface {
	Handle(ctx context.Context, id uint64) (*dto.NotificationDto, error)
}

type getNotiSvc struct {
	notiRepo      repo.INotificationRepo
	notiCacheRepo repo.INotificationCacheRepo
}

func NewGetNotiSvc(
	notiRepo repo.INotificationRepo,
	notiCacheRepo repo.INotificationCacheRepo,
) GetNotiSvc {
	return &getNotiSvc{
		notiRepo:      notiRepo,
		notiCacheRepo: notiCacheRepo,
	}
}

func (s *getNotiSvc) Handle(
	ctx context.Context,
	id uint64,
) (*dto.NotificationDto, error) {
	notification, err := s.notiRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return mapper.ToNotificationDto(notification), nil
}
