package service

import (
	"context"
	"simple-securities/config"
	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/application/mapper"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/domain/repo"
	"simple-securities/pkg/errors"
	"simple-securities/pkg/jwt"
)

type GetUserProfileSvc interface {
	Execute(ctx context.Context, accessToken string) (*dto.UserDto, error)
}

type getUserProfileSvc struct {
	userRepo repo.IUserRepo
}

func NewGetUserProfileSvc(userRepo repo.IUserRepo) GetUserProfileSvc {
	return &getUserProfileSvc{
		userRepo: userRepo,
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

	userExist, err := s.userRepo.FindById(ctx, userId)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeSystem, "Failed to get user by ID: %v", err)
	}
	if userExist == nil {
		return nil, model.ErrUserNotFound
	}

	return mapper.ToUserDto(userExist), nil
}
