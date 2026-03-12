package service

import (
	"context"
	"time"

	"simple-securities/common/client/grpc"
	"simple-securities/common/constants"
	noti "simple-securities/gen/notification/v1"

	"simple-securities/config"
	"simple-securities/internal/user/application/constant"
	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/application/mapper"
	"simple-securities/internal/user/application/util"
	"simple-securities/internal/user/domain/messaging"
	"simple-securities/internal/user/domain/repo"
	"simple-securities/pkg/errors"
	"simple-securities/pkg/jwt"
	"simple-securities/pkg/logger"

	"go.uber.org/zap"
)

type GetUserProfileSvc interface {
	Execute(ctx context.Context, accessToken string) (*dto.UserDto, error)
}

type getUserProfileSvc struct {
	notiClient *grpc.NotificationGrpcClient
	userRepo   repo.IUserRepo
	publisher  messaging.IEventPublisher
}

func NewGetUserProfileSvc(notiClient *grpc.NotificationGrpcClient, userRepo repo.IUserRepo, publisher messaging.IEventPublisher) GetUserProfileSvc {
	return &getUserProfileSvc{
		notiClient: notiClient,
		userRepo:   userRepo,
		publisher:  publisher,
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
		return nil, constant.ErrUserNotFound
	}

	s.notiClient.Send(ctx, &noti.SendRequest{
		UserId: userId,
		Title:  "Get User Profile Notification",
		Body:   "You have successfully get user profile.",
	})

	if s.publisher != nil {
		if pubErr := s.publisher.Publish(ctx, messaging.UserProfileViewed{
			UserID:    userExist.ID,
			UserUUID:  userExist.Uuid,
			Email:     userExist.Email,
			Timestamp: time.Now().UnixMilli(),
			RequestID: util.GetValueFromCtx(ctx, constants.RequestId),
		}); pubErr != nil {
			logger.Logger.Error("failed to publish UserProfileViewed event", zap.Error(pubErr))
		}
	}

	return mapper.ToUserDto(userExist), nil
}
