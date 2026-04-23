package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"simple-securities/internal/user/application/constant"
	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/domain/messaging"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/domain/repo"
	"simple-securities/pkg/db/txmanager"
	"simple-securities/pkg/logger"
)

type TopUpSvc interface {
	Execute(ctx context.Context, req *dto.TopUpReq) (*dto.TopUpResp, error)
}

type topUpSvc struct {
	walletRepo repo.ICreditWalletRepo
	txRepo     repo.ICreditTransactionRepo
	txManager  txmanager.TxManager
	redisLock  DistributedLock
	idemStore  IdempotencyStore
	publisher  messaging.IEventPublisher
}

func NewTopUpSvc(
	walletRepo repo.ICreditWalletRepo,
	txRepo repo.ICreditTransactionRepo,
	txManager txmanager.TxManager,
	redisLock DistributedLock,
	idemStore IdempotencyStore,
	publisher messaging.IEventPublisher,
) TopUpSvc {
	return &topUpSvc{
		walletRepo: walletRepo,
		txRepo:     txRepo,
		txManager:  txManager,
		redisLock:  redisLock,
		idemStore:  idemStore,
		publisher:  publisher,
	}
}

func (s *topUpSvc) Execute(ctx context.Context, req *dto.TopUpReq) (*dto.TopUpResp, error) {
	if req.Amount <= 0 {
		return nil, constant.ErrInvalidAmount
	}
	if req.WalletType.HasExpiry() && req.ExpireAtMs == nil {
		return nil, constant.ErrExpireAtRequired
	}

	// --- Idempotency fast-path ---
	if cached, err := s.idemStore.Get(ctx, req.IdempotencyKey); err == nil && cached != nil {
		return cached.(*dto.TopUpResp), nil
	}

	// --- Distributed lock per user ---
	lockKey := fmt.Sprintf(constant.LockKeyUserCredit, req.UserID)
	unlock, err := s.redisLock.Acquire(ctx, lockKey, 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer unlock()

	var expireAt *time.Time
	if req.ExpireAtMs != nil {
		t := time.UnixMilli(*req.ExpireAtMs)
		expireAt = &t
	}

	resp, err := txmanager.WithTxResult(ctx, s.txManager.GetTx(), func(tx *sqlx.Tx) (*dto.TopUpResp, error) {
		wallet, _ := s.walletRepo.FindByUserIDAndType(ctx, req.UserID, req.WalletType)
		if wallet == nil {
			wallet = model.NewCreditWallet(req.UserID, req.WalletType, expireAt)
		}

		balanceBefore := wallet.Balance
		wallet.AddBalance(req.Amount)

		saved, err := s.walletRepo.Save(ctx, tx, wallet)
		if err != nil {
			return nil, err
		}

		refID := req.ReferenceID
		idemKey := req.IdempotencyKey
		txn := model.NewCreditTransaction(
			req.UserID, saved.ID, req.WalletType,
			"TOP_UP", req.Amount, balanceBefore, saved.Balance,
			nil, &refID, &idemKey, nil,
		)
		if _, err := s.txRepo.Save(ctx, tx, txn); err != nil {
			return nil, err
		}

		return &dto.TopUpResp{WalletID: saved.Uuid, NewBalance: saved.Balance}, nil
	})
	if err != nil {
		return nil, err
	}

	_ = s.idemStore.Set(ctx, req.IdempotencyKey, resp, 24*time.Hour)

	if s.publisher != nil {
		if pubErr := s.publisher.Publish(ctx, messaging.CreditToppedUp{
			UserID:         req.UserID,
			WalletType:     string(req.WalletType),
			Amount:         req.Amount,
			NewBalance:     resp.NewBalance,
			ReferenceID:    req.ReferenceID,
			IdempotencyKey: req.IdempotencyKey,
			Timestamp:      time.Now().UnixMilli(),
		}); pubErr != nil {
			logger.Logger.Error("failed to publish CreditToppedUp", zap.Error(pubErr))
		}
	}

	return resp, nil
}
