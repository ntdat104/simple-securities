package mapper

import (
	noti "simple-securities/gen/notification/v1"
	"simple-securities/internal/notification/application/dto"
)

func ToCreateReq(req *noti.SendRequest) *dto.NotificationCreateReq {
	return &dto.NotificationCreateReq{
		UserID: req.UserId,
		Type:   req.Type,
		Title:  req.Title,
		Body:   req.Body,
	}
}

func ToNotification(notiDto *dto.NotificationDto) *noti.Notification {
	if notiDto == nil {
		return nil
	}
	return &noti.Notification{
		Id:        notiDto.ID,
		Uuid:      notiDto.Uuid,
		UserId:    notiDto.UserID,
		Type:      notiDto.Type,
		Title:     notiDto.Title,
		Body:      notiDto.Body,
		Read:      notiDto.Read,
		ReadAt:    notiDto.ReadAt,
		Viewed:    notiDto.Viewed,
		ViewedAt:  notiDto.ViewedAt,
		CreatedAt: notiDto.CreatedAt,
		UpdatedAt: notiDto.UpdatedAt,
	}
}

func ToNotifications(notiDtos []*dto.NotificationDto) []*noti.Notification {
	if notiDtos == nil {
		return nil
	}
	notis := make([]*noti.Notification, 0, len(notiDtos))
	for _, notiDto := range notiDtos {
		notis = append(notis, ToNotification(notiDto))
	}
	return notis
}
