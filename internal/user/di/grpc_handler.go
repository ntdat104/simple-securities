package di

import (
	"simple-securities/internal/user/handler/grpc"

	"github.com/google/wire"
)

// GrpcHandlerSet provides UserGrpcSvc with all fields auto-filled by wire.
// NewGrpcHandler is kept manual in main.go.
var GrpcHandlerSet = wire.NewSet(
	wire.Struct(new(grpc.UserGrpcSvc), "*"),
)
