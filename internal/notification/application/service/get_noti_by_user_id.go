package service

import (
	"context"
	"simple-securities/internal/notification/application/dto"
	"simple-securities/internal/notification/application/mapper"
)

func (s *notificationService) GetNotiByUserId(ctx context.Context, userId uint64, limit, offset uint32) ([]*dto.NotificationDto, error) {
	notifications, err := s.notificationRepo.GetByUserId(ctx, userId, limit, offset)
	if err != nil {
		return nil, err
	}

	return mapper.ToNotificationDtos(notifications), nil
}
