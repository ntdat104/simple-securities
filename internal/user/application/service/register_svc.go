package service

import (
	"context"
	"time"

	"simple-securities/common/constants"
	"simple-securities/config"
	"simple-securities/internal/user/application/constant"
	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/application/mapper"
	"simple-securities/internal/user/application/util"
	"simple-securities/internal/user/domain/messaging"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/domain/repo"
	"simple-securities/pkg/bcrypt"
	"simple-securities/pkg/db/txmanager"
	"simple-securities/pkg/errors"
	"simple-securities/pkg/logger"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

type RegisterSvc interface {
	Execute(ctx context.Context, req *dto.RegisterReq) (*dto.RegisterResp, error)
}

type registerSvc struct {
	tx              txmanager.TxManager
	userRepo        repo.IUserRepo
	userHistoryRepo repo.IUserHistoryRepo
	publisher       messaging.IEventPublisher
}

func NewRegisterSvc(tx txmanager.TxManager, userRepo repo.IUserRepo, userHistoryRepo repo.IUserHistoryRepo, publisher messaging.IEventPublisher) RegisterSvc {
	return &registerSvc{
		tx:              tx,
		userRepo:        userRepo,
		userHistoryRepo: userHistoryRepo,
		publisher:       publisher,
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

	if s.publisher != nil {
		if pubErr := s.publisher.Publish(ctx, messaging.UserRegistered{
			UserID:    userSaved.ID,
			UserUUID:  userSaved.Uuid,
			Email:     userSaved.Email,
			Timestamp: time.Now().UnixMilli(),
			RequestID: util.GetValueFromCtx(ctx, constants.RequestId),
		}); pubErr != nil {
			logger.Logger.Error("failed to publish UserRegistered event", zap.Error(pubErr))
		}
	}

	return &dto.RegisterResp{
		User:         mapper.ToUserDto(userSaved),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    config.GlobalConfig.Jwt.TokenType,
		Exp:          exp,
	}, nil
}
