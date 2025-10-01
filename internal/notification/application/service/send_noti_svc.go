package service

import (
	"context"
	"simple-securities/internal/notification/application/dto"
	"simple-securities/internal/notification/domain/model"
	"simple-securities/internal/notification/domain/repo"
)

type SendNotiSvc interface {
	Handle(ctx context.Context, req *dto.NotificationCreateReq) error
}

type sendNotiSvc struct {
	notiRepo      repo.INotificationRepo
	notiCacheRepo repo.INotificationCacheRepo
}

func NewSendNotiSvc(
	notiRepo repo.INotificationRepo,
	notiCacheRepo repo.INotificationCacheRepo,
) SendNotiSvc {
	return &sendNotiSvc{
		notiRepo:      notiRepo,
		notiCacheRepo: notiCacheRepo,
	}
}

func (s *sendNotiSvc) Handle(
	ctx context.Context,
	req *dto.NotificationCreateReq,
) error {
	_, err := s.notiRepo.Create(ctx, model.NewNotification(req.UserID, req.Type, req.Title, req.Body))
	if err != nil {
		return err
	}
	return nil
}
