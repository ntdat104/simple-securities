package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"simple-securities/internal/credit/application/constant"
	"simple-securities/internal/credit/application/dto"
	"simple-securities/internal/credit/domain/enum"
	"simple-securities/internal/credit/domain/messaging"
	"simple-securities/internal/credit/domain/model"
	"simple-securities/internal/credit/domain/repo"
	"simple-securities/pkg/db/txmanager"
	"simple-securities/pkg/logger"
)

type ReserveCreditsSvc interface {
	Execute(ctx context.Context, req *dto.ReserveCreditsReq) (*dto.ReserveCreditsResp, error)
}

type reserveCreditsSvc struct {
	walletRepo          repo.ICreditWalletRepo
	txRepo              repo.ICreditTransactionRepo
	reservationRepo     repo.ICreditReservationRepo
	reservationItemRepo repo.ICreditReservationItemRepo
	txManager           txmanager.TxManager
	redisLock           DistributedLock
	idemStore           IdempotencyStore
	publisher           messaging.IEventPublisher
}

func NewReserveCreditsSvc(
	walletRepo repo.ICreditWalletRepo,
	txRepo repo.ICreditTransactionRepo,
	reservationRepo repo.ICreditReservationRepo,
	reservationItemRepo repo.ICreditReservationItemRepo,
	txManager txmanager.TxManager,
	redisLock DistributedLock,
	idemStore IdempotencyStore,
	publisher messaging.IEventPublisher,
) ReserveCreditsSvc {
	return &reserveCreditsSvc{
		walletRepo:          walletRepo,
		txRepo:              txRepo,
		reservationRepo:     reservationRepo,
		reservationItemRepo: reservationItemRepo,
		txManager:           txManager,
		redisLock:           redisLock,
		idemStore:           idemStore,
		publisher:           publisher,
	}
}

func (s *reserveCreditsSvc) Execute(ctx context.Context, req *dto.ReserveCreditsReq) (*dto.ReserveCreditsResp, error) {
	if req.Amount <= 0 {
		return nil, constant.ErrInvalidAmount
	}

	// --- Idempotency fast-path (Redis) ---
	if cached, err := s.idemStore.Get(ctx, req.IdempotencyKey); err == nil && cached != nil {
		return cached.(*dto.ReserveCreditsResp), nil
	}

	// --- DB-level idempotency (reservation already created) ---
	if existing, _ := s.reservationRepo.FindByIdempotencyKey(ctx, req.IdempotencyKey); existing != nil {
		items, _ := s.reservationItemRepo.FindItemsByReservationID(ctx, existing.ID)
		return buildReserveResp(existing, items), nil
	}

	// --- Distributed lock per user (prevents concurrent balance races) ---
	lockKey := fmt.Sprintf(constant.LockKeyUserCredit, req.UserID)
	unlock, err := s.redisLock.Acquire(ctx, lockKey, 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer unlock()

	stockSymbol := req.StockSymbol
	requestID := req.RequestID

	resp, err := txmanager.WithTxResult(ctx, s.txManager.GetTx(), func(tx *sqlx.Tx) (*dto.ReserveCreditsResp, error) {
		// Wallets sorted: earliest expire_at first, PURCHASED (nil expire) last.
		wallets, err := s.walletRepo.FindByUserID(ctx, req.UserID)
		if err != nil {
			return nil, err
		}

		remaining := req.Amount
		var deductions []walletDeduction
		var updatedWallets []*model.CreditWallet
		var txns []*model.CreditTransaction

		for _, w := range wallets {
			if remaining <= 0 {
				break
			}
			if w.IsExpired() || w.Available() <= 0 {
				continue
			}
			take := minInt64(w.Available(), remaining)
			balBefore := w.Balance
			w.SoftReserve(take)
			remaining -= take
			deductions = append(deductions, walletDeduction{wallet: w, amount: take})
			updatedWallets = append(updatedWallets, w)
			idemKey := req.IdempotencyKey
			txns = append(txns, model.NewCreditTransaction(
				req.UserID, w.ID, w.WalletType,
				enum.TxTypeReserve, take, balBefore, w.Balance,
				nil, &stockSymbol, &idemKey, nil,
			))
		}

		if remaining > 0 {
			return nil, constant.ErrInsufficientCredits
		}

		if _, err := s.walletRepo.SaveAll(ctx, tx, updatedWallets); err != nil {
			return nil, err
		}

		reservation := model.NewCreditReservation(
			req.UserID, req.Amount, req.IdempotencyKey,
			&stockSymbol, &requestID, constant.ReservationTTL*time.Second,
		)
		saved, err := s.reservationRepo.Save(ctx, tx, reservation)
		if err != nil {
			return nil, err
		}

		var items []*model.CreditReservationItem
		for _, d := range deductions {
			items = append(items, &model.CreditReservationItem{
				ReservationID: saved.ID,
				WalletID:      d.wallet.ID,
				WalletType:    d.wallet.WalletType,
				Amount:        d.amount,
			})
			for i := range txns {
				if txns[i].WalletID == d.wallet.ID {
					txns[i].ReservationID = &saved.Uuid
				}
			}
		}
		if _, err := s.reservationItemRepo.SaveAll(ctx, tx, items); err != nil {
			return nil, err
		}
		if _, err := s.txRepo.SaveAll(ctx, tx, txns); err != nil {
			return nil, err
		}

		return buildReserveResp(saved, items), nil
	})
	if err != nil {
		return nil, err
	}

	_ = s.idemStore.Set(ctx, req.IdempotencyKey, resp, 30*time.Minute)

	if s.publisher != nil {
		if pubErr := s.publisher.Publish(ctx, messaging.CreditReserved{
			ReservationID: resp.ReservationID,
			UserID:        req.UserID,
			TotalAmount:   req.Amount,
			StockSymbol:   req.StockSymbol,
			RequestID:     req.RequestID,
			Timestamp:     time.Now().UnixMilli(),
		}); pubErr != nil {
			logger.Logger.Error("failed to publish CreditReserved", zap.Error(pubErr))
		}
	}

	return resp, nil
}

func buildReserveResp(r *model.CreditReservation, items []*model.CreditReservationItem) *dto.ReserveCreditsResp {
	deductions := make([]dto.WalletDeductionDto, 0, len(items))
	for _, item := range items {
		deductions = append(deductions, dto.WalletDeductionDto{
			WalletID:   fmt.Sprintf("%d", item.WalletID),
			WalletType: item.WalletType,
			Amount:     item.Amount,
		})
	}
	return &dto.ReserveCreditsResp{
		ReservationID:  r.Uuid,
		ReservedAmount: r.TotalAmount,
		Deductions:     deductions,
	}
}

type walletDeduction struct {
	wallet *model.CreditWallet
	amount int64
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
