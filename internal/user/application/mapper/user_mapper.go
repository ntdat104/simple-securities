package mapper

import (
	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/domain/model"
)

func ToUserDto(user *model.User) *dto.UserDto {
	return &dto.UserDto{
		ID:          user.ID,
		Uuid:        user.Uuid,
		Email:       user.Email,
		LastLoginAt: user.LastLoginAt,
		Status:      user.Status,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}
}
