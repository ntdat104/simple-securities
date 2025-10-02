package grpc

import (
	"context"
	crypto "simple-securities/gen/crypto/v1"
	"simple-securities/internal/crypto/application/dto"
	"simple-securities/internal/crypto/application/mapper"
	"simple-securities/internal/crypto/application/service"
)

type CryptoGrpcHandler struct {
	crypto.UnimplementedCryptoServiceServer
	getKlinesSvc     service.GetKlinesSvc
	getServerTimeSvc service.GetServerTimeSvc
}

func NewCryptoGrpcHandler(
	getKlinesSvc service.GetKlinesSvc,
	getServerTimeSvc service.GetServerTimeSvc,
) crypto.CryptoServiceServer {
	return &CryptoGrpcHandler{
		getKlinesSvc:     getKlinesSvc,
		getServerTimeSvc: getServerTimeSvc,
	}
}

func (h *CryptoGrpcHandler) GetServerTime(ctx context.Context, req *crypto.GetServerTimeRequest) (*crypto.GetServerTimeResponse, error) {
	result, err := h.getServerTimeSvc.Handle(ctx)
	if err != nil {
		return nil, err
	}
	return &crypto.GetServerTimeResponse{
		ServerTime: result.ServerTime,
	}, nil
}

func (h *CryptoGrpcHandler) GetKlines(ctx context.Context, req *crypto.GetKlinesRequest) (*crypto.GetKlinesResponse, error) {
	result, err := h.getKlinesSvc.Handle(ctx, &dto.GetKlinesReq{
		Symbol:   req.Symbol,
		Interval: req.Interval,
		Limit:    req.Limit,
	})
	if err != nil {
		return nil, err
	}
	return &crypto.GetKlinesResponse{
		Klines: mapper.ToKlines(result),
	}, nil
}
