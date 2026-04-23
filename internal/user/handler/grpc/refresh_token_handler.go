package grpc

import (
	"context"
	common "simple-securities/gen/common/v1"
	corev1 "simple-securities/gen/core/v1"
)

func (h *UserGrpcHandler) RefreshToken(ctx context.Context, req *corev1.RefreshTokenRequest) (*corev1.RefreshTokenResponse, error) {
	res, err := h.refreshTokenSvc.Execute(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	var lastLoginAt int64

	if res.User.LastLoginAt != nil {
		lastLoginAt = res.User.LastLoginAt.UnixMilli()
	}

	return &corev1.RefreshTokenResponse{
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
