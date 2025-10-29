package service

import (
	"context"
	"simple-securities/config"
	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/application/mapper"
	"simple-securities/internal/user/application/util"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/domain/repo"
	"simple-securities/pkg/bcrypt"
	"simple-securities/pkg/errors"
)

type RegisterSvc interface {
	Execute(ctx context.Context, req *dto.RegisterReq) (*dto.RegisterResp, error)
}

type registerSvc struct {
	userRepo repo.IUserRepo
}

func NewRegisterSvc(userRepo repo.IUserRepo) RegisterSvc {
	return &registerSvc{
		userRepo: userRepo,
	}
}

func (s *registerSvc) Execute(ctx context.Context, req *dto.RegisterReq) (*dto.RegisterResp, error) {
	if req.Email == "" {
		return nil, model.ErrInvalidUserEmail
	}

	if req.Password == "" {
		return nil, model.ErrUserPasswordMissing
	}

	userExist, _ := s.userRepo.FindByEmail(ctx, req.Email)
	if userExist != nil {
		return nil, model.ErrUserEmailTaken
	}

	hashedPassword, err := bcrypt.HashPassword(req.Password)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeBusiness, "Failed to hash password: %v", err)
	}

	newUser, err := model.NewUser(req.Email, string(hashedPassword))
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeValidation, "Fail to create new User: %v", err)
	}

	userSaved, err := s.userRepo.Save(ctx, newUser)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeSystem, "Internal system error during user save: %v", err)
	}

	accessToken, exp, err := util.GenerateJwtToken(
		userSaved.ID,
		userSaved.Uuid,
		userSaved.Email,
		config.GlobalConfig.Jwt.SecretKey,
		config.GlobalConfig.Jwt.AccessTokenExpiry,
	)

	refreshToken, exp, err := util.GenerateJwtToken(
		userSaved.ID,
		userSaved.Uuid,
		userSaved.Email,
		config.GlobalConfig.Jwt.SecretKey,
		config.GlobalConfig.Jwt.RefreshTokenExpiry,
	)

	userSaved.RefreshToken = refreshToken
	_, err = s.userRepo.Save(ctx, userSaved)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeBusiness, "Failed to generate access token: %v", err)
	}

	return &dto.RegisterResp{
		User:         mapper.ToUserDto(userSaved),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    config.GlobalConfig.Jwt.TokenType,
		Exp:          exp,
	}, nil
}
