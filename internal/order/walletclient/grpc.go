package walletclient

import (
	"context"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	walletv1 "simple-securities/gen/wallet/v1"
	"simple-securities/pkg/grpcx"
)

// GRPC talks to the wallet over gRPC.
type GRPC struct {
	conn *grpc.ClientConn
	cl   walletv1.WalletServiceClient
}

// NewGRPC dials the wallet gRPC endpoint (connection is established lazily).
func NewGRPC(addr string) (*GRPC, error) {
	conn, err := grpcx.Dial(addr)
	if err != nil {
		return nil, err
	}
	return &GRPC{conn: conn, cl: walletv1.NewWalletServiceClient(conn)}, nil
}

// LockFunds implements Client.
func (g *GRPC) LockFunds(ctx context.Context, orderID, userID, asset string, amount decimal.Decimal) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	resp, err := g.cl.LockFunds(ctx, &walletv1.LockFundsRequest{
		OrderId: orderID, UserId: userID, Asset: asset, Amount: amount.String(),
	})
	if err != nil {
		return false, err
	}
	return resp.GetAlreadyLocked(), nil
}

// UnlockFunds implements Client.
func (g *GRPC) UnlockFunds(ctx context.Context, orderID, reason string) (decimal.Decimal, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	resp, err := g.cl.UnlockFunds(ctx, &walletv1.UnlockFundsRequest{OrderId: orderID, Reason: reason})
	if err != nil {
		return decimal.Zero, err
	}
	return parseDec(resp.GetReleased())
}

// GetLock implements Client.
func (g *GRPC) GetLock(ctx context.Context, orderID string) (Lock, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	resp, err := g.cl.GetLock(ctx, &walletv1.GetLockRequest{OrderId: orderID})
	if status.Code(err) == codes.NotFound {
		return Lock{}, nil
	}
	if err != nil {
		return Lock{}, err
	}
	return lockFromProto(resp)
}

// Close implements Client.
func (g *GRPC) Close() error { return g.conn.Close() }
