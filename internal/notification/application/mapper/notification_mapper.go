package mapper

import (
	"simple-securities/internal/notification/application/dto"
	"simple-securities/internal/notification/domain/model"
)

func ToNotificationDto(input *model.Notification) *dto.NotificationDto {
	if input == nil {
		return nil
	}
	var readAt, viewedAt, updatedAt, createdAt uint64
	if input.ReadAt != nil {
		readAt = uint64(input.ReadAt.Unix())
	}
	if input.ViewedAt != nil {
		viewedAt = uint64(input.ViewedAt.Unix())
	}
	if !input.UpdatedAt.IsZero() {
		updatedAt = uint64(input.UpdatedAt.Unix())
	}
	if !input.CreatedAt.IsZero() {
		createdAt = uint64(input.CreatedAt.Unix())
	}
	return &dto.NotificationDto{
		ID:        input.ID,
		Uuid:      input.Uuid,
		UserID:    input.UserID,
		Type:      input.Type,
		Title:     input.Title,
		Body:      input.Body,
		Read:      input.Read,
		ReadAt:    readAt,
		Viewed:    input.Viewed,
		ViewedAt:  viewedAt,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

func ToNotificationDtos(inputs []*model.Notification) []*dto.NotificationDto {
	if inputs == nil {
		return nil
	}
	dtos := make([]*dto.NotificationDto, 0, len(inputs))
	for _, input := range inputs {
		dtos = append(dtos, ToNotificationDto(input))
	}
	return dtos
}
