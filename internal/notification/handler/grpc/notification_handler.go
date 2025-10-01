package grpc

import (
	"context"
	noti "simple-securities/gen/notification/v1"
	"simple-securities/internal/notification/application/mapper"
	"simple-securities/internal/notification/application/service"
)

type NotificationGrpcHandler struct {
	noti.UnimplementedNotificationServiceServer
	sendNotiSvc        service.SendNotiSvc
	getNotiSvc         service.GetNotiSvc
	getNotiByUserIdSvc service.GetNotiByUserIdSvc
}

func NewNotificationGrpcHandler(
	sendNotiSvc service.SendNotiSvc,
	getNotiSvc service.GetNotiSvc,
	getNotiByUserIdSvc service.GetNotiByUserIdSvc,
) noti.NotificationServiceServer {
	return &NotificationGrpcHandler{
		sendNotiSvc:        sendNotiSvc,
		getNotiSvc:         getNotiSvc,
		getNotiByUserIdSvc: getNotiByUserIdSvc,
	}
}

func (h *NotificationGrpcHandler) Send(ctx context.Context, req *noti.SendRequest) (*noti.SendResponse, error) {
	err := h.sendNotiSvc.Handle(ctx, mapper.ToCreateReq(req))
	if err != nil {
		return &noti.SendResponse{Success: false}, err
	}
	return &noti.SendResponse{Success: true}, nil
}

func (h *NotificationGrpcHandler) Get(ctx context.Context, req *noti.GetRequest) (*noti.GetResponse, error) {
	notiDto, err := h.getNotiSvc.Handle(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &noti.GetResponse{Notification: mapper.ToNotification(notiDto)}, nil
}

func (h *NotificationGrpcHandler) GetByUserId(ctx context.Context, req *noti.GetByUserIdRequest) (*noti.GetByUserIdResponse, error) {
	notiDtos, err := h.getNotiByUserIdSvc.Handle(ctx, req.UserId, req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	return &noti.GetByUserIdResponse{Notifications: mapper.ToNotifications(notiDtos)}, nil
}
