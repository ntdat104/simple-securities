package service

import (
	"context"
	"simple-securities/internal/notification/application/dto"
	"simple-securities/internal/notification/domain/repo"
)

type NotificationService interface {
	SendNoti(ctx context.Context, req *dto.NotificationCreateReq) error
	GetNoti(ctx context.Context, id uint64) (*dto.NotificationDto, error)
	GetNotiByUserId(ctx context.Context, userId uint64, limit, offset uint32) ([]*dto.NotificationDto, error)
}

type notificationService struct {
	notificationRepo repo.INotificationRepo
}

func NewNotificationService(
	notificationRepo repo.INotificationRepo,
) NotificationService {
	return &notificationService{
		notificationRepo: notificationRepo,
	}
}
