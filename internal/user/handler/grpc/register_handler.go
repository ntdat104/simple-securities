package grpc

import (
	"context"
	common "simple-securities/gen/common/v1"
	corev1 "simple-securities/gen/core/v1"
	"simple-securities/internal/user/application/dto"
)

func (h *UserGrpcHandler) Register(ctx context.Context, req *corev1.RegisterRequest) (*corev1.RegisterResponse, error) {
	res, err := h.registerSvc.Execute(ctx, &dto.RegisterReq{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	return &corev1.RegisterResponse{
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
