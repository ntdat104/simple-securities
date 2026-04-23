package grpc

import (
	corev1 "simple-securities/gen/core/v1"

	"simple-securities/internal/user/application/service"
)

type UserGrpcSvc struct {
	RegisterSvc       service.RegisterSvc
	LoginSvc          service.LoginSvc
	RefreshTokenSvc   service.RefreshTokenSvc
	GetUserProfileSvc service.GetUserProfileSvc
}

type UserGrpcHandler struct {
	corev1.UnimplementedUserServiceServer
	registerSvc       service.RegisterSvc
	loginSvc          service.LoginSvc
	refreshTokenSvc   service.RefreshTokenSvc
	getUserProfileSvc service.GetUserProfileSvc
}

func NewUserGrpcHandler(svc UserGrpcSvc) corev1.UserServiceServer {
	return &UserGrpcHandler{
		registerSvc:       svc.RegisterSvc,
		loginSvc:          svc.LoginSvc,
		refreshTokenSvc:   svc.RefreshTokenSvc,
		getUserProfileSvc: svc.GetUserProfileSvc,
	}
}
