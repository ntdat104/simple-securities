package grpc

import (
	"context"

	corev1 "simple-securities/gen/core/v1"
	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/application/service"
	"simple-securities/internal/user/domain/enum"
)

type CreditGrpcHandler struct {
	corev1.UnimplementedCreditServiceServer
	topUpSvc      service.TopUpSvc
	reserveSvc    service.ReserveCreditsSvc
	commitSvc     service.CommitReservationSvc
	rollbackSvc   service.RollbackReservationSvc
	getBalanceSvc service.GetCreditBalanceSvc
}

func NewCreditGrpcHandler(
	topUpSvc service.TopUpSvc,
	reserveSvc service.ReserveCreditsSvc,
	commitSvc service.CommitReservationSvc,
	rollbackSvc service.RollbackReservationSvc,
	getBalanceSvc service.GetCreditBalanceSvc,
) *CreditGrpcHandler {
	return &CreditGrpcHandler{
		topUpSvc:      topUpSvc,
		reserveSvc:    reserveSvc,
		commitSvc:     commitSvc,
		rollbackSvc:   rollbackSvc,
		getBalanceSvc: getBalanceSvc,
	}
}

func (h *CreditGrpcHandler) TopUp(ctx context.Context, req *corev1.TopUpRequest) (*corev1.TopUpResponse, error) {
	resp, err := h.topUpSvc.Execute(ctx, &dto.TopUpReq{
		UserID:         parseUint64(req.UserId),
		WalletType:     enum.WalletType(req.WalletType.String()),
		Amount:         req.Amount,
		ExpireAtMs:     req.ExpireAt,
		IdempotencyKey: req.IdempotencyKey,
		ReferenceID:    req.ReferenceId,
	})
	if err != nil {
		return nil, toGrpcError(err)
	}
	return &corev1.TopUpResponse{WalletId: resp.WalletID, NewBalance: resp.NewBalance}, nil
}

func (h *CreditGrpcHandler) ReserveCredits(ctx context.Context, req *corev1.ReserveCreditsRequest) (*corev1.ReserveCreditsResponse, error) {
	resp, err := h.reserveSvc.Execute(ctx, &dto.ReserveCreditsReq{
		UserID:         parseUint64(req.UserId),
		Amount:         req.Amount,
		IdempotencyKey: req.IdempotencyKey,
		StockSymbol:    req.StockSymbol,
		RequestID:      req.RequestId,
	})
	if err != nil {
		return nil, toGrpcError(err)
	}

	var deductions []*corev1.WalletDeduction
	for _, d := range resp.Deductions {
		deductions = append(deductions, &corev1.WalletDeduction{
			WalletId:   d.WalletID,
			WalletType: toProtoWalletType(d.WalletType),
			Amount:     d.Amount,
		})
	}
	return &corev1.ReserveCreditsResponse{
		ReservationId:  resp.ReservationID,
		ReservedAmount: resp.ReservedAmount,
		Deductions:     deductions,
	}, nil
}

func (h *CreditGrpcHandler) CommitReservation(ctx context.Context, req *corev1.CommitReservationRequest) (*corev1.CommitReservationResponse, error) {
	resp, err := h.commitSvc.Execute(ctx, &dto.CommitReservationReq{
		ReservationID:  req.ReservationId,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, toGrpcError(err)
	}
	return &corev1.CommitReservationResponse{
		Success:       resp.Success,
		TotalDeducted: resp.TotalDeducted,
	}, nil
}

func (h *CreditGrpcHandler) RollbackReservation(ctx context.Context, req *corev1.RollbackReservationRequest) (*corev1.RollbackReservationResponse, error) {
	resp, err := h.rollbackSvc.Execute(ctx, &dto.RollbackReservationReq{
		ReservationID: req.ReservationId,
		Reason:        req.Reason,
	})
	if err != nil {
		return nil, toGrpcError(err)
	}
	return &corev1.RollbackReservationResponse{Success: resp.Success}, nil
}

func (h *CreditGrpcHandler) GetCreditBalance(ctx context.Context, req *corev1.GetCreditBalanceRequest) (*corev1.GetCreditBalanceResponse, error) {
	resp, err := h.getBalanceSvc.Execute(ctx, &dto.GetCreditBalanceReq{
		UserID: parseUint64(req.UserId),
	})
	if err != nil {
		return nil, toGrpcError(err)
	}

	var wallets []*corev1.WalletBalance
	for _, w := range resp.Wallets {
		wallets = append(wallets, &corev1.WalletBalance{
			WalletId:   w.WalletID,
			WalletType: toProtoWalletType(w.WalletType),
			Balance:    w.Balance,
			Reserved:   w.Reserved,
			Available:  w.Available,
			ExpireAt:   w.ExpireAtMs,
		})
	}
	return &corev1.GetCreditBalanceResponse{
		UserId:         req.UserId,
		TotalAvailable: resp.TotalAvailable,
		Wallets:        wallets,
	}, nil
}
