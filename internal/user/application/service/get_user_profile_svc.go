package service

import (
	"context"

	noti "simple-securities/gen/notification/v1"

	"simple-securities/config"
	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/application/mapper"
	"simple-securities/internal/user/client/grpc"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/domain/repo"
	"simple-securities/pkg/errors"
	"simple-securities/pkg/jwt"
)

type GetUserProfileSvc interface {
	Execute(ctx context.Context, accessToken string) (*dto.UserDto, error)
}

type getUserProfileSvc struct {
	notiClient *grpc.NotificationGrpcClient
	userRepo   repo.IUserRepo
}

func NewGetUserProfileSvc(notiClient *grpc.NotificationGrpcClient, userRepo repo.IUserRepo) GetUserProfileSvc {
	return &getUserProfileSvc{
		notiClient: notiClient,
		userRepo:   userRepo,
	}
}

func (s *getUserProfileSvc) Execute(ctx context.Context, accessToken string) (*dto.UserDto, error) {
	claims, err := jwt.ParseWithClaims(accessToken, config.GlobalConfig.Jwt.SecretKey)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeUnauthorized, "Parse with claims failed: %v", err)
	}

	userId := claims.UserID
	if userId == 0 {
		return nil, errors.New(errors.ErrorTypeUnauthorized, "Token claims missing user id.")
	}

	email := claims.Email
	if email == "" {
		return nil, errors.New(errors.ErrorTypeUnauthorized, "Token claims missing email.")
	}

	userUuid := claims.UserUUID
	if userUuid == "" {
		return nil, errors.New(errors.ErrorTypeUnauthorized, "Token claims missing user uuid.")
	}

	userExist, err := s.userRepo.FindByIdAndEmailAndUuid(ctx, userId, email, userUuid)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeSystem, "Failed to get user by ID: %v", err)
	}
	if userExist == nil {
		return nil, model.ErrUserNotFound
	}

	s.notiClient.Send(ctx, &noti.SendRequest{
		UserId: userId,
		Title:  "Get User Profile Notification",
		Body:   "You have successfully get user profile.",
	})

	return mapper.ToUserDto(userExist), nil
}
