package logger

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	KeyRequestID     = "request-id"
	KeyUserID        = "user-id"
	KeyUserUUID      = "user-uuid"
	KeyUserEmail     = "user-email"
	KeyApiKey        = "api-key"
	KeyApiSecret     = "api-secret"
	KeySignature     = "signature"
	KeyAuthorization = "authorization"
)

func ForwardMetadataInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	mdIn, _ := metadata.FromIncomingContext(ctx)

	pairs := []string{}
	keysToForward := []string{
		KeyRequestID,
		KeyUserID,
		KeyUserUUID,
		KeyUserEmail,
		KeyApiKey,
		KeyApiSecret,
		KeySignature,
		KeyAuthorization,
	}

	for _, key := range keysToForward {
		var val string
		if v, ok := ctx.Value(key).(string); ok && v != "" {
			val = v
		} else if vals := mdIn.Get(key); len(vals) > 0 {
			val = vals[0]
		}

		if val != "" {
			pairs = append(pairs, key, val)
		}
	}

	if len(pairs) > 0 {
		ctx = metadata.AppendToOutgoingContext(ctx, pairs...)
	}

	return invoker(ctx, method, req, reply, cc, opts...)
}
