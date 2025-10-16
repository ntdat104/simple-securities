package middleware

import (
	"context"
	"simple-securities/config"
	"simple-securities/pkg/logger"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func LoggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	start := time.Now()

	resp, err = handler(ctx, req)

	end := time.Now()
	duration := end.Sub(start)

	if err != nil {
		logger.Logger.Error("gRPC request",
			zap.String("env", string(config.GlobalConfig.Env)),
			zap.String("app_name", config.GlobalConfig.App.Name),
			zap.String("method", info.FullMethod),
			zap.String("status", "failed"),
			zap.Time("start_time", start),
			zap.Time("end_time", end),
			zap.Duration("duration", duration),
			zap.Any("request", req),
			zap.Error(err),
		)
	} else {
		logger.Logger.Info("gRPC request",
			zap.String("env", string(config.GlobalConfig.Env)),
			zap.String("app_name", config.GlobalConfig.App.Name),
			zap.String("method", info.FullMethod),
			zap.String("status", "success"),
			zap.Time("start_time", start),
			zap.Time("end_time", end),
			zap.Duration("duration", duration),
			zap.Any("request", req),
			zap.Any("response", resp),
		)
	}

	return resp, err
}
