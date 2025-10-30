package grpc

import (
	"context"
	common "simple-securities/gen/common/v1"
	userpb "simple-securities/gen/user/v1"
	"simple-securities/internal/user/application/dto"
)

func (h *UserGrpcHandler) Login(ctx context.Context, req *userpb.LoginRequest) (*userpb.LoginResponse, error) {
	res, err := h.loginSvc.Execute(ctx, &dto.LoginReq{
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
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		TokenType:    res.TokenType,
		Exp:          res.Exp,
	}, nil
}
