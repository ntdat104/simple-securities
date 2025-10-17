package grpc

import (
	"context"
	"strings"

	common "simple-securities/gen/common/v1"
	userpb "simple-securities/gen/user/v1"

	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/application/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type UserGrpcHandler struct {
	userpb.UnimplementedUserServiceServer
	userRegisterSvc   service.UserRegisterSvc
	userLoginSvc      service.UserLoginSvc
	userGetProfileSvc service.UserGetProfileSvc
}

func NewUserGrpcHandler(
	userRegisterSvc service.UserRegisterSvc,
	userLoginSvc service.UserLoginSvc,
	userGetProfileSvc service.UserGetProfileSvc,
) userpb.UserServiceServer {
	return &UserGrpcHandler{
		userRegisterSvc:   userRegisterSvc,
		userLoginSvc:      userLoginSvc,
		userGetProfileSvc: userGetProfileSvc,
	}
}

func (h *UserGrpcHandler) Register(ctx context.Context, req *userpb.RegisterRequest) (*userpb.RegisterResponse, error) {
	res, err := h.userRegisterSvc.Handle(ctx, &dto.UserRegisterReq{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	return &userpb.RegisterResponse{
		User: &common.UserDto{
			Id:          res.User.ID,
			Uuid:        res.User.Uuid,
			Email:       res.User.Email,
			LastLoginAt: nil,
			Status:      string(res.User.Status),
		},
		AccessToken: res.AccessToken,
		TokenType:   res.TokenType,
		Exp:         res.Exp,
	}, nil
}

func (h *UserGrpcHandler) Login(ctx context.Context, req *userpb.LoginRequest) (*userpb.LoginResponse, error) {
	res, err := h.userLoginSvc.Handle(ctx, &dto.UserLoginReq{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}

	var lastLoginAt int64

	if res.User.LastLoginAt != nil {
		lastLoginAt = res.User.LastLoginAt.UnixMilli()
	}

	return &userpb.LoginResponse{
		User: &common.UserDto{
			Id:          res.User.ID,
			Uuid:        res.User.Uuid,
			Email:       res.User.Email,
			LastLoginAt: &lastLoginAt,
			Status:      string(res.User.Status),
		},
		AccessToken: res.AccessToken,
		TokenType:   res.TokenType,
		Exp:         res.Exp,
	}, nil
}

func (h *UserGrpcHandler) GetUserProfile(ctx context.Context, req *userpb.GetUserProfileRequest) (*userpb.GetUserProfileResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "Authorization metadata not found.")
	}

	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return nil, status.Error(codes.Unauthenticated, "Authorization header missing.")
	}

	authHeader := authHeaders[0]
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, status.Error(codes.Unauthenticated, "Invalid authorization scheme. Expected 'Bearer <token>'.")
	}

	accessToken := strings.TrimPrefix(authHeader, "Bearer ")
	if accessToken == "" {
		return nil, status.Error(codes.Unauthenticated, "Access token is empty.")
	}

	res, err := h.userGetProfileSvc.Handle(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	var lastLoginAt int64

	if res.LastLoginAt != nil {
		lastLoginAt = res.LastLoginAt.UnixMilli()
	}

	return &userpb.GetUserProfileResponse{
		User: &common.UserDto{
			Id:          res.ID,
			Uuid:        res.Uuid,
			Email:       res.Email,
			LastLoginAt: &lastLoginAt,
			Status:      string(res.Status),
		},
	}, nil
}
