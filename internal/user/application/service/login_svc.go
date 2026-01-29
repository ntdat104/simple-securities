package service

import (
	"context"
	"log"

	"simple-securities/common/client/grpc"
	noti "simple-securities/gen/notification/v1"

	"simple-securities/config"

	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/application/mapper"
	"simple-securities/internal/user/application/util"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/domain/repo"

	"simple-securities/pkg/bcrypt"
	"simple-securities/pkg/datetime"
	"simple-securities/pkg/errors"
)

type LoginSvc interface {
	Execute(ctx context.Context, req *dto.LoginReq) (*dto.LoginResp, error)
}

type loginSvc struct {
	notiClient *grpc.NotificationGrpcClient
	userRepo   repo.IUserRepo
}

func NewLoginSvc(notiClient *grpc.NotificationGrpcClient, userRepo repo.IUserRepo) LoginSvc {
	return &loginSvc{
		notiClient: notiClient,
		userRepo:   userRepo,
	}
}

func (s *loginSvc) Execute(ctx context.Context, req *dto.LoginReq) (*dto.LoginResp, error) {
	if req.Email == "" {
		return nil, model.ErrInvalidUserEmail
	}

	if req.Password == "" {
		return nil, model.ErrUserPasswordMissing
	}

	userExist, _ := s.userRepo.FindByEmail(ctx, req.Email)
	if userExist == nil {
		return nil, model.ErrUserNotFound
	}

	err := bcrypt.ComparePassword(userExist.HashedPassword, req.Password)
	if err != nil {
		return nil, errors.New(errors.ErrorTypeUnauthorized, "Invalid email or password")
	}

	accessToken, exp, err := util.GenerateJwtToken(
		userExist.ID,
		userExist.Uuid,
		userExist.Email,
		config.GlobalConfig.Jwt.SecretKey,
		config.GlobalConfig.Jwt.AccessTokenExpiry,
	)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeBusiness, "Failed to generate access token: %v", err)
	}

	refreshToken, exp, err := util.GenerateJwtToken(
		userExist.ID,
		userExist.Uuid,
		userExist.Email,
		config.GlobalConfig.Jwt.SecretKey,
		config.GlobalConfig.Jwt.RefreshTokenExpiry,
	)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeBusiness, "Failed to generate refresh token: %v", err)
	}

	now := datetime.Now()
	userExist.LastLoginAt = &now
	userExist.RefreshToken = refreshToken
	userSaved, err := s.userRepo.Save(ctx, userExist)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeBusiness, "Failed to save last login: %v", err)
	}

	log.Printf("user %+v", userSaved)

	s.notiClient.Send(ctx, &noti.SendRequest{
		UserId: userSaved.ID,
		Title:  "Login Notification",
		Body:   "You have successfully logged in.",
	})

	return &dto.LoginResp{
		User:         mapper.ToUserDto(userSaved),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    config.GlobalConfig.Jwt.TokenType,
		Exp:          exp,
	}, nil
}
