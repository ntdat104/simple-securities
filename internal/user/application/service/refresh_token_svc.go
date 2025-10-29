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
	"time"
)

type RefreshTokenSvc interface {
	Execute(ctx context.Context, refreshToken string) (*dto.RefreshTokenResp, error)
}

type refreshTokenSvc struct {
	userRepo repo.IUserRepo
}

func NewRefreshTokenSvc(userRepo repo.IUserRepo) RefreshTokenSvc {
	return &refreshTokenSvc{
		userRepo: userRepo,
	}
}

func (s *refreshTokenSvc) Execute(ctx context.Context, refreshToken string) (*dto.RefreshTokenResp, error) {
	claims, err := jwt.ParseWithClaims(refreshToken, config.GlobalConfig.Jwt.SecretKey)
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

	if userExist.RefreshToken != refreshToken {
		return nil, errors.New(errors.ErrorTypeUnauthorized, "Invalid refresh token.")
	}

	accessToken, exp, err := jwt.GenerateJwtToken(
		userExist.ID,
		userExist.Uuid,
		userExist.Email,
		config.GlobalConfig.Jwt.SecretKey,
		config.GlobalConfig.Jwt.AccessTokenExpiry,
	)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeBusiness, "Failed to generate access token: %v", err)
	}

	newRefreshToken, exp, err := jwt.GenerateJwtToken(
		userExist.ID,
		userExist.Uuid,
		userExist.Email,
		config.GlobalConfig.Jwt.SecretKey,
		config.GlobalConfig.Jwt.RefreshTokenExpiry,
	)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeBusiness, "Failed to generate refresh token: %v", err)
	}

	userExist.RefreshToken = newRefreshToken
	userExist.UpdatedAt = time.Now()
	userExist.UpdatedBy = userExist.ID
	_, err = s.userRepo.Save(ctx, userExist)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeSystem, "Failed to update user refresh token: %v", err)
	}

	return &dto.RefreshTokenResp{
		User:         mapper.ToUserDto(userExist),
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		TokenType:    config.GlobalConfig.Jwt.TokenType,
		Exp:          exp,
	}, nil
}
