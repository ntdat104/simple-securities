// Package grpcapi exposes the wallet use cases over gRPC (and, via grpc-gateway, HTTP/JSON).
package grpcapi

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	walletv1 "simple-securities/gen/wallet/v1"
	"simple-securities/internal/wallet/app"
	"simple-securities/internal/wallet/store"
)

// Server implements walletv1.WalletServiceServer.
type Server struct {
	walletv1.UnimplementedWalletServiceServer
	svc *app.Service
}

// NewServer returns the gRPC server implementation.
func NewServer(svc *app.Service) *Server { return &Server{svc: svc} }

// GetBalances returns all balances of a user.
func (s *Server) GetBalances(ctx context.Context, req *walletv1.GetBalancesRequest) (*walletv1.GetBalancesResponse, error) {
	rows, err := s.svc.GetBalances(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	out := make([]*walletv1.Balance, len(rows))
	for i, r := range rows {
		out[i] = &walletv1.Balance{Asset: r.Asset, Free: r.Free.String(), Locked: r.Locked.String()}
	}
	return &walletv1.GetBalancesResponse{Balances: out}, nil
}

// Deposit credits a simulated deposit.
func (s *Server) Deposit(ctx context.Context, req *walletv1.DepositRequest) (*walletv1.LedgerEntry, error) {
	e, err := s.svc.Deposit(ctx, req.GetUserId(), req.GetAsset(), req.GetAmount(), req.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	return toEntry(e), nil
}

// Withdraw debits a simulated withdrawal.
func (s *Server) Withdraw(ctx context.Context, req *walletv1.WithdrawRequest) (*walletv1.LedgerEntry, error) {
	e, err := s.svc.Withdraw(ctx, req.GetUserId(), req.GetAsset(), req.GetAmount(), req.GetAddress(),
		req.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	return toEntry(e), nil
}

// TransferToFutures moves USDT from the spot wallet to the futures account (asynchronous credit).
func (s *Server) TransferToFutures(ctx context.Context, req *walletv1.TransferToFuturesRequest) (*walletv1.LedgerEntry, error) {
	e, err := s.svc.TransferToFutures(ctx, req.GetUserId(), req.GetAsset(), req.GetAmount(), req.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	return toEntry(e), nil
}

// LockFunds reserves funds for an order.
func (s *Server) LockFunds(ctx context.Context, req *walletv1.LockFundsRequest) (*walletv1.LockFundsResponse, error) {
	r, err := s.svc.LockFunds(ctx, req.GetOrderId(), req.GetUserId(), req.GetAsset(), req.GetAmount())
	if err != nil {
		return nil, err
	}
	return &walletv1.LockFundsResponse{OrderId: r.OrderID, Asset: r.Asset, Amount: r.Amount.String(),
		AlreadyLocked: r.AlreadyLocked}, nil
}

// UnlockFunds releases an order's remaining lock.
func (s *Server) UnlockFunds(ctx context.Context, req *walletv1.UnlockFundsRequest) (*walletv1.UnlockFundsResponse, error) {
	released, err := s.svc.UnlockFunds(ctx, req.GetOrderId(), req.GetReason())
	if err != nil {
		return nil, err
	}
	return &walletv1.UnlockFundsResponse{Released: released.String()}, nil
}

// GetLock returns an order's lock.
func (s *Server) GetLock(ctx context.Context, req *walletv1.GetLockRequest) (*walletv1.GetLockResponse, error) {
	l, found, err := s.svc.GetLock(ctx, req.GetOrderId())
	if err != nil {
		return nil, err
	}
	if !found {
		return &walletv1.GetLockResponse{Found: false, OrderId: req.GetOrderId()}, nil
	}
	return &walletv1.GetLockResponse{Found: true, OrderId: l.OrderID, UserId: l.UserID, Asset: l.Asset,
		Amount: l.Amount.String(), Remaining: l.Remaining.String(), Status: l.Status}, nil
}

// ListLedger returns ledger entries newest first.
func (s *Server) ListLedger(ctx context.Context, req *walletv1.ListLedgerRequest) (*walletv1.ListLedgerResponse, error) {
	entries, err := s.svc.ListLedger(ctx, req.GetUserId(), int(req.GetLimit()), req.GetBeforeId())
	if err != nil {
		return nil, err
	}
	out := make([]*walletv1.LedgerEntry, len(entries))
	for i, e := range entries {
		out[i] = toEntry(e)
	}
	return &walletv1.ListLedgerResponse{Entries: out}, nil
}

func toEntry(e store.LedgerEntry) *walletv1.LedgerEntry {
	return &walletv1.LedgerEntry{
		Id:          e.ID,
		UserId:      e.UserID,
		Asset:       e.Asset,
		DeltaFree:   e.DeltaFree.String(),
		DeltaLocked: e.DeltaLocked.String(),
		FreeAfter:   optDec(e.FreeAfter),
		LockedAfter: optDec(e.LockedAfter),
		RefType:     e.RefType,
		RefId:       e.RefID,
		CreatedAt:   e.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func optDec(d *decimal.Decimal) string {
	if d == nil {
		return ""
	}
	return d.String()
}
