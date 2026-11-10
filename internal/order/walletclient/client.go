// Package walletclient is the order service's client of the wallet service. Two transports
// (gRPC and HTTP/JSON through the wallet's grpc-gateway) implement the same interface; errors are
// gRPC status errors in both cases.
package walletclient

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	walletv1 "simple-securities/gen/wallet/v1"
	"simple-securities/pkg/config"
)

// callTimeout bounds every wallet call.
const callTimeout = 3 * time.Second

// Lock statuses.
const (
	LockActive   = "ACTIVE"
	LockReleased = "RELEASED"
)

// Lock is an order's funds lock as seen by the wallet.
type Lock struct {
	Found     bool
	OrderID   string
	UserID    string
	Asset     string
	Amount    decimal.Decimal
	Remaining decimal.Decimal
	Status    string // ACTIVE | RELEASED
}

// Client is what the order service needs from the wallet.
type Client interface {
	// LockFunds moves amount of asset from free to locked for the order. Idempotent on orderID.
	LockFunds(ctx context.Context, orderID, userID, asset string, amount decimal.Decimal) (alreadyLocked bool, err error)
	// UnlockFunds releases whatever remains locked for the order. Idempotent.
	UnlockFunds(ctx context.Context, orderID, reason string) (released decimal.Decimal, err error)
	// GetLock returns the order's lock; Found is false when none exists.
	GetLock(ctx context.Context, orderID string) (Lock, error)
	// Close releases transport resources.
	Close() error
}

// New builds the client selected by WALLET_TRANSPORT (grpc|http, default grpc).
func New() (Client, error) {
	switch transport := config.String("WALLET_TRANSPORT", "grpc"); transport {
	case "grpc":
		return NewGRPC(config.String("WALLET_GRPC_ADDR", "localhost:9102"))
	case "http":
		return NewHTTP(config.String("WALLET_HTTP_URL", "http://localhost:8102")), nil
	default:
		return nil, fmt.Errorf("unknown WALLET_TRANSPORT %q (want grpc|http)", transport)
	}
}

func lockFromProto(r *walletv1.GetLockResponse) (Lock, error) {
	if !r.GetFound() {
		return Lock{}, nil
	}
	l := Lock{Found: true, OrderID: r.GetOrderId(), UserID: r.GetUserId(), Asset: r.GetAsset(), Status: r.GetStatus()}
	var err error
	if l.Amount, err = parseDec(r.GetAmount()); err != nil {
		return Lock{}, err
	}
	if l.Remaining, err = parseDec(r.GetRemaining()); err != nil {
		return Lock{}, err
	}
	return l, nil
}

func parseDec(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, fmt.Errorf("wallet returned invalid decimal %q: %w", s, err)
	}
	return d, nil
}
