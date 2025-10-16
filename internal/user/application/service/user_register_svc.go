package service

import (
	"context"
	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/application/mapper"
	"simple-securities/internal/user/application/util"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/domain/repo"
	"simple-securities/pkg/errors"

	"golang.org/x/crypto/bcrypt"
)

type UserRegisterSvc interface {
	Handle(ctx context.Context, req *dto.UserRegisterReq) (*dto.UserRegisterResp, error)
}

type userRegisterSvc struct {
	userRepo repo.IUserRepo
}

func NewUserRegisterSvc(userRepo repo.IUserRepo) UserRegisterSvc {
	return &userRegisterSvc{
		userRepo: userRepo,
	}
}

func (s *userRegisterSvc) Handle(ctx context.Context, req *dto.UserRegisterReq) (*dto.UserRegisterResp, error) {
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

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
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

	accessToken, exp, err := util.GenerateAccessToken(
		userSaved.ID,
		userSaved.Uuid,
		userSaved.Email,
		"secret-key",
	)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeBusiness, "Failed to generate access token: %v", err)
	}

	return &dto.UserRegisterResp{
		User:        mapper.ToUserDto(userSaved),
		AccessToken: accessToken,
		TokenType:   "Bearer",
		Exp:         exp,
	}, nil
}
