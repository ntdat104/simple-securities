package service

import (
	"time"

	"simple-securities/application/dto"
)

type SystemService interface {
	GetTime() *dto.SystemTimeRes
}

type systemService struct{}

func NewSystemService() SystemService {
	return &systemService{}
}

func (s *systemService) GetTime() *dto.SystemTimeRes {
	return &dto.SystemTimeRes{
		SystemTime: time.Now().UnixMilli(),
	}
}
