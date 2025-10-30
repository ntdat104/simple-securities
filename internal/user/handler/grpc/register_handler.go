package grpc

import (
	"context"
	common "simple-securities/gen/common/v1"
	userpb "simple-securities/gen/user/v1"
	"simple-securities/internal/user/application/dto"
)

func (h *UserGrpcHandler) Register(ctx context.Context, req *userpb.RegisterRequest) (*userpb.RegisterResponse, error) {
	res, err := h.registerSvc.Execute(ctx, &dto.RegisterReq{
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
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		TokenType:    res.TokenType,
		Exp:          res.Exp,
	}, nil
}
