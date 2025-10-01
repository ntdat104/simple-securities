package service

import (
	"context"
	"simple-securities/internal/notification/application/dto"
	"simple-securities/internal/notification/domain/model"
)

func (s *notificationService) SendNoti(ctx context.Context, req *dto.NotificationCreateReq) error {
	_, err := s.notificationRepo.Create(ctx, model.NewNotification(req.UserID, req.Type, req.Title, req.Body))
	if err != nil {
		return err
	}

	return nil
}
