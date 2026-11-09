// Package grpcx builds gRPC servers and clients with the platform's standard interceptors.
package grpcx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"time"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/resolver/dns"
	"google.golang.org/grpc/status"
)

var serverMetrics = grpcprom.NewServerMetrics(grpcprom.WithServerHandlingTimeHistogram())

func init() {
	prometheus.MustRegister(serverMetrics)
	// Re-resolve DNS at most every 2s (default 30s) so clients follow a restarted/rescheduled pod
	// (new IP) quickly instead of failing against a stale address.
	dns.SetMinResolutionInterval(2 * time.Second)
}

// NewServer returns a server with recovery, logging, metrics, health and reflection.
func NewServer(log *zap.Logger) *grpc.Server {
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			recoveryInterceptor(log),
			serverMetrics.UnaryServerInterceptor(),
			loggingInterceptor(log),
		),
		grpc.KeepaliveParams(keepalive.ServerParameters{MaxConnectionIdle: 5 * time.Minute, Time: time.Minute}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{MinTime: 10 * time.Second, PermitWithoutStream: true}),
	)
	healthpb.RegisterHealthServer(srv, health.NewServer())
	reflection.Register(srv)
	return srv
}

// Serve listens on addr until ctx is cancelled, then stops gracefully.
func Serve(ctx context.Context, srv *grpc.Server, addr string, log *zap.Logger) error {
	serverMetrics.InitializeMetrics(srv)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(lis) }()
	log.Info("gRPC server listening", zap.String("addr", addr))
	select {
	case <-ctx.Done():
		done := make(chan struct{})
		go func() { srv.GracefulStop(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			srv.Stop()
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return err
	}
}

// Dial creates a client connection with round-robin LB over DNS and keepalive.
// Production: replace insecure credentials with mTLS (or a service mesh).
func Dial(target string) (*grpc.ClientConn, error) {
	return grpc.NewClient("dns:///"+target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig":[{"round_robin":{}}]}`),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 30 * time.Second, Timeout: 5 * time.Second, PermitWithoutStream: true}),
	)
}

func recoveryInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic in handler", zap.String("method", info.FullMethod), zap.Any("panic", r),
					zap.ByteString("stack", debug.Stack()))
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return h(ctx, req)
	}
}

func loggingInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := h(ctx, req)
		code := status.Code(err)
		fields := []zap.Field{zap.String("method", info.FullMethod), zap.String("code", code.String()),
			zap.Duration("latency", time.Since(start))}
		switch code {
		case codes.OK:
			log.Debug("grpc", fields...)
		case codes.Internal, codes.Unknown, codes.DataLoss, codes.Unavailable:
			// Hide internal details from callers, keep them in logs.
			log.Error("grpc", append(fields, zap.Error(err))...)
			if _, ok := status.FromError(err); !ok {
				err = status.Error(codes.Internal, "internal error")
			}
		default:
			log.Info("grpc", append(fields, zap.Error(err))...)
		}
		return resp, err
	}
}
