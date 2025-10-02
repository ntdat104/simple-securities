package grpc

import (
	"io"
	"log"
	"net"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type GrpcServerConfig struct {
	Port             uint32
	KeepaliveParams  keepalive.ServerParameters
	KeepalivePolicy  keepalive.EnforcementPolicy
	UnaryInterceptor grpc.UnaryServerInterceptor
}

type GRPCServer interface {
	Start(serviceRegister func(server *grpc.Server))
	io.Closer
}

type gRPCServer struct {
	grpcServer *grpc.Server
	config     GrpcServerConfig
}

func NewGrpcServer(config GrpcServerConfig) (GRPCServer, error) {
	options, err := buildOptions(config)
	if err != nil {
		return nil, err
	}

	server := grpc.NewServer(options...)

	return &gRPCServer{
		config:     config,
		grpcServer: server,
	}, nil
}

func buildOptions(config GrpcServerConfig) ([]grpc.ServerOption, error) {
	return []grpc.ServerOption{
		grpc.KeepaliveParams(buildKeepaliveParams(config.KeepaliveParams)),
		grpc.KeepaliveEnforcementPolicy(buildKeepalivePolicy(config.KeepalivePolicy)),
		grpc.UnaryInterceptor(config.UnaryInterceptor),
	}, nil
}

func buildKeepalivePolicy(config keepalive.EnforcementPolicy) keepalive.EnforcementPolicy {
	return keepalive.EnforcementPolicy{
		MinTime:             config.MinTime * time.Second,
		PermitWithoutStream: config.PermitWithoutStream,
	}
}

func buildKeepaliveParams(config keepalive.ServerParameters) keepalive.ServerParameters {
	return keepalive.ServerParameters{
		MaxConnectionIdle:     config.MaxConnectionIdle * time.Second,
		MaxConnectionAge:      config.MaxConnectionAge * time.Second,
		MaxConnectionAgeGrace: config.MaxConnectionAgeGrace * time.Second,
		Time:                  config.Time * time.Second,
		Timeout:               config.Timeout * time.Second,
	}
}

func (g gRPCServer) Start(serviceRegister func(server *grpc.Server)) {
	grpcListener, err := net.Listen("tcp", ":"+strconv.Itoa(int(g.config.Port)))
	if err != nil {
		log.Fatalf("failed to start grpc server: %v", err)
	}

	serviceRegister(g.grpcServer)

	if err := g.grpcServer.Serve(grpcListener); err != nil {
		log.Fatalf("failed to serve grpc server: %v", err)
	}
}

func (g gRPCServer) Close() error {
	// Only log on error, but GracefulStop() doesn’t return error
	// so nothing to log unless we change to Stop() and wrap it
	g.grpcServer.GracefulStop()
	return nil
}
