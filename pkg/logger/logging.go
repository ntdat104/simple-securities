package logger

import (
	"context"
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

	requestID := getMetadataValue(md, "request-id", uuid.NewGoogleUUID())
	apiKey := getMetadataValue(md, "api-key", "")
	apiSecret := getMetadataValue(md, "api-secret", "")
	signature := getMetadataValue(md, "signature", "")
	authHeader := getMetadataValue(md, "authorization", "")

	header := metadata.Pairs(
		"request-id", requestID,
		"api-key", apiKey,
		"api-secret", apiSecret,
		"signature", signature,
		"authorization", authHeader,
	)
	grpc.SetHeader(ctx, header)

	claims := extractClaims(authHeader)

	ctx = context.WithValue(ctx, "request-id", requestID)
	ctx = context.WithValue(ctx, "user-id", claims.UserID)
	ctx = context.WithValue(ctx, "user-uuid", claims.UserUUID)
	ctx = context.WithValue(ctx, "user-email", claims.Email)
	ctx = context.WithValue(ctx, "api-key", apiKey)
	ctx = context.WithValue(ctx, "api-secret", apiSecret)
	ctx = context.WithValue(ctx, "signature", signature)
	ctx = context.WithValue(ctx, "authorization", authHeader)

	start := datetime.Now()
	resp, err = handler(ctx, req)
	end := datetime.Now()
	duration := end.Sub(start)

	fields := []zap.Field{
		zap.String("request_id", requestID),
		zap.String("env", string(config.GlobalConfig.Env)),
		zap.String("app_name", config.GlobalConfig.App.Name),
		zap.String("method", info.FullMethod),
		zap.Uint64("user_id", claims.UserID),
		zap.String("user_uuid", claims.UserUUID),
		zap.String("user_email", claims.Email),
		zap.String("api_key", apiKey),
		zap.String("api_secret", apiSecret),
		zap.String("signature", signature),
		zap.Time("start_time", start),
		zap.Time("end_time", end),
		zap.Duration("duration", duration),
		zap.Any("request", req),
	}

	if err != nil {
		fields = append(fields, zap.Error(err), zap.String("status", "failed"))
		Logger.Error("gRPC request failed", fields...)
	} else {
		fields = append(fields, zap.Any("response", resp), zap.String("status", "success"))
		Logger.Info("gRPC request success", fields...)
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
