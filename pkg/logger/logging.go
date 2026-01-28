package logger

import (
	"context"
	"log"
	"simple-securities/config"
	"simple-securities/pkg/datetime"
	"simple-securities/pkg/uuid"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func LoggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	md, _ := metadata.FromIncomingContext(ctx)

	var requestId string
	requestIds := md.Get("request-id")
	if len(requestIds) != 0 {
		requestId = requestIds[0]
	} else {
		requestId = uuid.NewGoogleUUID()
	}

	for k, v := range md {
		log.Println(k, v)
	}

	start := datetime.Now()

	resp, err = handler(ctx, req)

	end := datetime.Now()
	duration := end.Sub(start)

	if err != nil {
		Logger.Error("gRPC request",
			zap.String("request_id", requestId),
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
		Logger.Info("gRPC request",
			zap.String("request_id", requestId),
			zap.String("env", string(config.GlobalConfig.Env)),
			zap.String("app_name", config.GlobalConfig.App.Name),
			zap.String("method", info.FullMethod),
			zap.String("status", "success"),
			zap.Time("start_time", start),
			zap.Time("end_time", end),
			zap.Duration("duration", duration),
			zap.Any("request", req),
			// zap.Any("response", resp),
		)
	}

	return resp, err
}
