package service

import (
	"context"
	"fmt"
	"log"

	"simple-securities/common/client/grpc"
	noti "simple-securities/gen/notification/v1"

	"simple-securities/config"

	"simple-securities/internal/user/application/constant"
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
		return nil, constant.ErrInvalidUserEmail
	}

	if req.Password == "" {
		return nil, constant.ErrUserPasswordMissing
	}

	val, _ := s.userRepo.CountTotal(ctx)
	log.Println(val)

	val3, _ := s.userRepo.FindAllByPageAndSize(ctx, 0, 1)
	for _, val := range val3 {
		log.Printf("%+v", val)
	}

	userExist, _ := s.userRepo.FindByEmail(ctx, req.Email)
	if userExist == nil {
		return nil, constant.ErrUserNotFound
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
	userSaved, err := s.userRepo.Save(ctx, nil, userExist)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeBusiness, "Failed to save last login: %v", err)
	}

	log.Printf("user %+v", userSaved)

	users := []*model.User{}
	for i := range 1000 {
		newUser, _ := model.NewUser(fmt.Sprintf("abc-%d@gmail.com", i), "123123")
		users = append(users, newUser)
	}

	s.userRepo.SaveAll(ctx, nil, users)

	x, cursor, _ := s.userRepo.FindAllByCursor(ctx, "", 10)
	for _, v := range x {
		log.Printf("%+v ", v)
	}

	x, cursor, _ = s.userRepo.FindAllByCursor(ctx, cursor, 10)
	for _, v := range x {
		log.Printf("%+v ", v)
	}

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
