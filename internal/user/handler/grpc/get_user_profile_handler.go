package grpc

import (
	"context"
	common "simple-securities/gen/common/v1"
	corev1 "simple-securities/gen/core/v1"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func (h *UserGrpcHandler) GetUserProfile(ctx context.Context, req *corev1.GetUserProfileRequest) (*corev1.GetUserProfileResponse, error) {
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

	res, err := h.getUserProfileSvc.Execute(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	var lastLoginAt int64

	if res.LastLoginAt != nil {
		lastLoginAt = res.LastLoginAt.UnixMilli()
	}

	return &corev1.GetUserProfileResponse{
		User: &common.UserDto{
			Id:          res.ID,
			Uuid:        res.Uuid,
			Email:       res.Email,
			LastLoginAt: &lastLoginAt,
			Status:      string(res.Status),
		},
	}, nil
}
