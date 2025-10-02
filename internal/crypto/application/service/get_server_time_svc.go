package service

import (
	"context"
	"simple-securities/internal/crypto/application/dto"
	"time"
)

type GetServerTimeSvc interface {
	Handle(ctx context.Context) (*dto.ServerTimeDto, error)
}

type getServerTimeSvc struct{}

func NewGetServerTimeSvc() GetServerTimeSvc {
	return &getServerTimeSvc{}
}

func (s *getServerTimeSvc) Handle(ctx context.Context) (*dto.ServerTimeDto, error) {
	result := &dto.ServerTimeDto{
		ServerTime: time.Now().Unix(),
	}
	return result, nil
}
