package service

import (
	"context"
	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/application/mapper"
	"simple-securities/internal/user/application/util"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/domain/repo"
	"simple-securities/pkg/errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type UserLoginSvc interface {
	Handle(ctx context.Context, req *dto.UserLoginReq) (*dto.UserLoginResp, error)
}

type userLoginSvc struct {
	userRepo repo.IUserRepo
}

func NewUserLoginSvc(userRepo repo.IUserRepo) UserLoginSvc {
	return &userLoginSvc{
		userRepo: userRepo,
	}
}

func (s *userLoginSvc) Handle(ctx context.Context, req *dto.UserLoginReq) (*dto.UserLoginResp, error) {
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

	err := bcrypt.CompareHashAndPassword([]byte(userExist.HashedPassword), []byte(req.Password))
	if err != nil {
		return nil, errors.New(errors.ErrorTypeUnauthorized, "Invalid email or password")
	}

	now := time.Now()
	userExist.LastLoginAt = &now
	userSaved, err := s.userRepo.Save(ctx, userExist)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeBusiness, "Failed to save last login: %v", err)
	}

	accessToken, exp, err := util.GenerateAccessToken(
		userSaved.ID,
		userSaved.Uuid,
		userSaved.Email,
		"secret-key",
	)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeBusiness, "Failed to generate access token: %v", err)
	}

	return &dto.UserLoginResp{
		User:        mapper.ToUserDto(userSaved),
		AccessToken: accessToken,
		TokenType:   "Bearer",
		Exp:         exp,
	}, nil
}
