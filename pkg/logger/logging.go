package logger

import (
	"context"
	"simple-securities/common/constants"
	"simple-securities/config"
	"simple-securities/pkg/datetime"
	"simple-securities/pkg/jwt"
	"simple-securities/pkg/uuid"
	"strings"

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

	requestID := getMetadataValue(md, constants.RequestId, uuid.NewGoogleUUID())
	apiKey := getMetadataValue(md, constants.ApiKey, "")
	apiSecret := getMetadataValue(md, constants.ApiSecret, "")
	signature := getMetadataValue(md, constants.Signature, "")
	authHeader := getMetadataValue(md, constants.Authorization, "")

	header := metadata.Pairs(
		constants.RequestId, requestID,
		constants.ApiKey, apiKey,
		constants.ApiSecret, apiSecret,
		constants.Signature, signature,
		constants.Authorization, authHeader,
	)
	grpc.SetHeader(ctx, header)

	claims := extractClaims(authHeader)

	ctx = context.WithValue(ctx, constants.RequestId, requestID)
	ctx = context.WithValue(ctx, constants.UserId, claims.UserID)
	ctx = context.WithValue(ctx, constants.UserUuid, claims.UserUUID)
	ctx = context.WithValue(ctx, constants.UserEmail, claims.Email)
	ctx = context.WithValue(ctx, constants.ApiKey, apiKey)
	ctx = context.WithValue(ctx, constants.ApiSecret, apiSecret)
	ctx = context.WithValue(ctx, constants.Signature, signature)
	ctx = context.WithValue(ctx, constants.Authorization, authHeader)

	start := datetime.Now()
	resp, err = handler(ctx, req)
	end := datetime.Now()
	duration := end.Sub(start)

	fields := []zap.Field{
		zap.String(constants.RequestId, requestID),
		zap.String(constants.Env, string(config.GlobalConfig.Env)),
		zap.String(constants.AppName, config.GlobalConfig.App.Name),
		zap.String(constants.Method, info.FullMethod),
		zap.Uint64(constants.UserId, claims.UserID),
		zap.String(constants.UserUuid, claims.UserUUID),
		zap.String(constants.UserEmail, claims.Email),
		zap.String(constants.ApiKey, apiKey),
		zap.String(constants.ApiSecret, apiSecret),
		zap.String(constants.Signature, signature),
		zap.Time(constants.StartTime, start),
		zap.Time(constants.EndTime, end),
		zap.Duration(constants.Duration, duration),
		zap.Any(constants.Request, req),
	}

	if err != nil {
		fields = append(fields, zap.Error(err), zap.String(constants.Status, constants.Failed))
		Logger.Error("❌ gRPC request failed", fields...)
	} else {
		fields = append(fields, zap.Any("response", resp), zap.String(constants.Status, constants.Success))
		Logger.Info("✅ gRPC request success", fields...)
	}

	return resp, err
}

// Helper: Extract metadata with a fallback default
func getMetadataValue(md metadata.MD, key, defaultValue string) string {
	if values := md.Get(key); len(values) > 0 {
		return values[0]
	}
	return defaultValue
}

// Helper: Parse JWT safely
func extractClaims(authHeader string) *jwt.Claims {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return &jwt.Claims{}
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := jwt.ParseWithClaims(token, config.GlobalConfig.Jwt.SecretKey)
	if err != nil {
		return &jwt.Claims{}
	}

	return claims
}
