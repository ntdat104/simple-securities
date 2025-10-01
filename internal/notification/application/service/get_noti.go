package service

import (
	"context"
	"simple-securities/internal/notification/application/dto"
	"simple-securities/internal/notification/application/mapper"
)

func (s *notificationService) GetNoti(ctx context.Context, id uint64) (*dto.NotificationDto, error) {
	notification, err := s.notificationRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return mapper.ToNotificationDto(notification), nil
}
