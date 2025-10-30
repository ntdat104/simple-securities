package grpc

import (
	"context"
	common "simple-securities/gen/common/v1"
	userpb "simple-securities/gen/user/v1"
)

func (h *UserGrpcHandler) RefreshToken(ctx context.Context, req *userpb.RefreshTokenRequest) (*userpb.RefreshTokenResponse, error) {
	res, err := h.refreshTokenSvc.Execute(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	var lastLoginAt int64

	if res.User.LastLoginAt != nil {
		lastLoginAt = res.User.LastLoginAt.UnixMilli()
	}

	return &userpb.RefreshTokenResponse{
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
