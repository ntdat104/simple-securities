package walletclient

import (
	"context"
	"net/http"
	"net/url"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	walletv1 "simple-securities/gen/wallet/v1"
	"simple-securities/pkg/httpx"
)

// HTTP talks to the wallet's grpc-gateway (paths from proto/wallet/v1/wallet.proto).
type HTTP struct {
	cl *httpx.Client
}

// NewHTTP returns a client for base (e.g. http://wallet:8102).
func NewHTTP(base string) *HTTP { return &HTTP{cl: httpx.NewClient(base)} }

// LockFunds implements Client.
func (h *HTTP) LockFunds(ctx context.Context, orderID, userID, asset string, amount decimal.Decimal) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	var resp walletv1.LockFundsResponse
	err := h.cl.Do(ctx, http.MethodPost, "/v1/wallet/locks", nil, &walletv1.LockFundsRequest{
		OrderId: orderID, UserId: userID, Asset: asset, Amount: amount.String(),
	}, &resp)
	if err != nil {
		return false, err
	}
	return resp.GetAlreadyLocked(), nil
}

// UnlockFunds implements Client.
func (h *HTTP) UnlockFunds(ctx context.Context, orderID, reason string) (decimal.Decimal, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	var resp walletv1.UnlockFundsResponse
	err := h.cl.Do(ctx, http.MethodPost, "/v1/wallet/locks/"+url.PathEscape(orderID)+"/release", nil,
		&walletv1.UnlockFundsRequest{OrderId: orderID, Reason: reason}, &resp)
	if err != nil {
		return decimal.Zero, err
	}
	return parseDec(resp.GetReleased())
}

// GetLock implements Client.
func (h *HTTP) GetLock(ctx context.Context, orderID string) (Lock, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	var resp walletv1.GetLockResponse
	err := h.cl.Do(ctx, http.MethodGet, "/v1/wallet/locks/"+url.PathEscape(orderID), nil, nil, &resp)
	if status.Code(err) == codes.NotFound {
		return Lock{}, nil
	}
	if err != nil {
		return Lock{}, err
	}
	return lockFromProto(&resp)
}

// Close implements Client.
func (h *HTTP) Close() error { return nil }
