package service

import (
	"context"
	"simple-securities/config"
	"simple-securities/internal/user/application/constant"
	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/application/mapper"
	"simple-securities/internal/user/application/util"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/domain/repo"
	"simple-securities/pkg/bcrypt"
	"simple-securities/pkg/db/txmanager"
	"simple-securities/pkg/errors"

	"github.com/jmoiron/sqlx"
)

type RegisterSvc interface {
	Execute(ctx context.Context, req *dto.RegisterReq) (*dto.RegisterResp, error)
}

type registerSvc struct {
	tx              txmanager.TxManager
	userRepo        repo.IUserRepo
	userHistoryRepo repo.IUserHistoryRepo
}

func NewRegisterSvc(tx txmanager.TxManager, userRepo repo.IUserRepo, userHistoryRepo repo.IUserHistoryRepo) RegisterSvc {
	return &registerSvc{
		tx:              tx,
		userRepo:        userRepo,
		userHistoryRepo: userHistoryRepo,
	}
}

func (s *registerSvc) Execute(ctx context.Context, req *dto.RegisterReq) (*dto.RegisterResp, error) {
	if req.Email == "" {
		return nil, constant.ErrInvalidUserEmail
	}

	if req.Password == "" {
		return nil, constant.ErrUserPasswordMissing
	}

	userExist, _ := s.userRepo.FindByEmail(ctx, req.Email)
	if userExist != nil {
		return nil, constant.ErrUserEmailTaken
	}

	hashedPassword, err := bcrypt.HashPassword(req.Password)
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeBusiness, "Failed to hash password: %v", err)
	}

	newUser, err := model.NewUser(req.Email, string(hashedPassword))
	if err != nil {
		return nil, errors.Newf(errors.ErrorTypeValidation, "Fail to create new User: %v", err)
	}

	userSaved, err := s.userRepo.Save(ctx, nil, newUser)
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

	// txmanager.WithTxResult(ctx, s.tx.GetTx(), func(tx *sqlx.Tx) (*model.User, error) {
	// 	val, err := s.userRepo.Save(ctx, tx, userSaved)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	input, err := model.NewUserHistory(userSaved.Email, string(hashedPassword))
	// 	_, err = s.userHistoryRepo.Save(ctx, tx, input)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	return val, nil
	// })

	s.tx.WithTx(ctx, func(tx *sqlx.Tx) error {
		_, err := s.userRepo.Save(ctx, tx, userSaved)
		if err != nil {
			return err
		}

		newUserHistory, err := model.NewUserHistory(userSaved.Email, string(hashedPassword))
		_, err = s.userHistoryRepo.Save(ctx, tx, newUserHistory)
		if err != nil {
			return err
		}

		return nil
	})

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
