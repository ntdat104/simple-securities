package grpc

import (
	userpb "simple-securities/gen/user/v1"

	"simple-securities/internal/user/application/service"
)

type UserGrpcSvc struct {
	RegisterSvc       service.RegisterSvc
	LoginSvc          service.LoginSvc
	RefreshTokenSvc   service.RefreshTokenSvc
	GetUserProfileSvc service.GetUserProfileSvc
}

type UserGrpcHandler struct {
	userpb.UnimplementedUserServiceServer
	registerSvc       service.RegisterSvc
	loginSvc          service.LoginSvc
	refreshTokenSvc   service.RefreshTokenSvc
	getUserProfileSvc service.GetUserProfileSvc
}

func NewUserGrpcHandler(svc UserGrpcSvc) userpb.UserServiceServer {
	return &UserGrpcHandler{
		registerSvc:       svc.RegisterSvc,
		loginSvc:          svc.LoginSvc,
		refreshTokenSvc:   svc.RefreshTokenSvc,
		getUserProfileSvc: svc.GetUserProfileSvc,
	}
}
