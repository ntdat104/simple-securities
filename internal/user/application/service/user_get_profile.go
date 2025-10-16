package service

import (
	"context"
	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/application/mapper"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/domain/repo"
	"simple-securities/pkg/errors"

	"github.com/golang-jwt/jwt/v5"
)

type UserGetProfileSvc interface {
	Handle(ctx context.Context, accessToken string) (*dto.UserDto, error)
}

type userGetProfileSvc struct {
	userRepo repo.IUserRepo
}

func NewUserGetProfileSvc(userRepo repo.IUserRepo) UserGetProfileSvc {
	return &userGetProfileSvc{
		userRepo: userRepo,
	}
}

func (s *userGetProfileSvc) Handle(ctx context.Context, accessToken string) (*dto.UserDto, error) {
	type UserClaims struct {
		UserID   uint64 `json:"user_id"`
		UserUUID string `json:"user_uuid"`
		Email    string `json:"email"`
		jwt.RegisteredClaims
	}

	claims := &UserClaims{}
	token, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (interface{}, error) {
		// Kiểm tra thuật toán ký
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New(errors.ErrorTypeBusiness, "unexpected signing method")
		}
		return []byte("secret-key"), nil
	})

	if err != nil || !token.Valid {
		return nil, errors.Newf(errors.ErrorTypeUnauthorized, "Invalid or expired token: %v", err)
	}

	userId := claims.UserID
	if userId == 0 {
		return nil, errors.New(errors.ErrorTypeUnauthorized, "Token claims missing user UUID.")
	}

	userExist, _ := s.userRepo.FindById(ctx, userId)
	if userExist == nil {
		return nil, model.ErrUserNotFound
	}

	return mapper.ToUserDto(userExist), nil
}
