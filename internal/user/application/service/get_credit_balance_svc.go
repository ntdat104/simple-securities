package service

import (
	"context"

	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/domain/repo"
)

type GetCreditBalanceSvc interface {
	Execute(ctx context.Context, req *dto.GetCreditBalanceReq) (*dto.GetCreditBalanceResp, error)
}

type getCreditBalanceSvc struct {
	walletRepo repo.ICreditWalletRepo
}

func NewGetCreditBalanceSvc(walletRepo repo.ICreditWalletRepo) GetCreditBalanceSvc {
	return &getCreditBalanceSvc{walletRepo: walletRepo}
}

func (s *getCreditBalanceSvc) Execute(ctx context.Context, req *dto.GetCreditBalanceReq) (*dto.GetCreditBalanceResp, error) {
	wallets, err := s.walletRepo.FindByUserID(ctx, req.UserID)
	if err != nil {
		return nil, err
	}

	var totalAvailable int64
	balances := make([]dto.WalletBalanceDto, 0, len(wallets))
	for _, w := range wallets {
		if w.IsExpired() {
			continue
		}
		avail := w.Available()
		totalAvailable += avail

		var expireAtMs *int64
		if w.ExpireAt != nil {
			ms := w.ExpireAt.UnixMilli()
			expireAtMs = &ms
		}
		balances = append(balances, dto.WalletBalanceDto{
			WalletID:   w.Uuid,
			WalletType: w.WalletType,
			Balance:    w.Balance,
			Reserved:   w.Reserved,
			Available:  avail,
			ExpireAtMs: expireAtMs,
		})
	}

	return &dto.GetCreditBalanceResp{
		UserID:         req.UserID,
		TotalAvailable: totalAvailable,
		Wallets:        balances,
	}, nil
}
