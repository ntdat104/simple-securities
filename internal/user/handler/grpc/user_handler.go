package grpc

import (
	"context"
	userpb "simple-securities/gen/user/v1"
	"simple-securities/internal/user/application/service"
)

type UserGrpcHandler struct {
	userpb.UnimplementedUserServiceServer
	userSvc service.UserSvc
}

func NewUserGrpcHandler(userSvc service.UserSvc) userpb.UserServiceServer {
	return &UserGrpcHandler{
		userSvc: userSvc,
	}
}

func (h *UserGrpcHandler) Register(ctx context.Context, req *userpb.RegisterRequest) (*userpb.RegisterResponse, error) {
	return h.userSvc.Register(ctx, req)
}

func (h *UserGrpcHandler) Login(ctx context.Context, req *userpb.LoginRequest) (*userpb.LoginResponse, error) {
	return h.userSvc.Login(ctx, req)
}

func (h *UserGrpcHandler) GetUserProfile(ctx context.Context, req *userpb.GetUserProfileRequest) (*userpb.GetUserProfileResponse, error) {
	return h.userSvc.GetUserProfile(ctx, req)
}
