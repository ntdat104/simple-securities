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

type CommitReservationSvc interface {
	Execute(ctx context.Context, req *dto.CommitReservationReq) (*dto.CommitReservationResp, error)
}

type commitReservationSvc struct {
	walletRepo          repo.ICreditWalletRepo
	txRepo              repo.ICreditTransactionRepo
	reservationRepo     repo.ICreditReservationRepo
	reservationItemRepo repo.ICreditReservationItemRepo
	txManager           txmanager.TxManager
	redisLock           DistributedLock
	idemStore           IdempotencyStore
	publisher           messaging.IEventPublisher
}

func NewCommitReservationSvc(
	walletRepo repo.ICreditWalletRepo,
	txRepo repo.ICreditTransactionRepo,
	reservationRepo repo.ICreditReservationRepo,
	reservationItemRepo repo.ICreditReservationItemRepo,
	txManager txmanager.TxManager,
	redisLock DistributedLock,
	idemStore IdempotencyStore,
	publisher messaging.IEventPublisher,
) CommitReservationSvc {
	return &commitReservationSvc{
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

func (s *commitReservationSvc) Execute(ctx context.Context, req *dto.CommitReservationReq) (*dto.CommitReservationResp, error) {
	// --- Idempotency fast-path ---
	if cached, err := s.idemStore.Get(ctx, req.IdempotencyKey); err == nil && cached != nil {
		return cached.(*dto.CommitReservationResp), nil
	}

	reservation, err := s.reservationRepo.FindByUuid(ctx, req.ReservationID)
	if err != nil || reservation == nil {
		return nil, constant.ErrReservationNotFound
	}
	if reservation.Status != enum.ReservationPending {
		return nil, constant.ErrReservationNotPending
	}
	if time.Now().After(reservation.ExpiresAt) {
		return nil, constant.ErrReservationExpired
	}

	// --- Distributed lock per reservation to prevent concurrent commits ---
	lockKey := fmt.Sprintf(constant.LockKeyReservation, reservation.Uuid)
	unlock, err := s.redisLock.Acquire(ctx, lockKey, 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer unlock()

	items, err := s.reservationItemRepo.FindItemsByReservationID(ctx, reservation.ID)
	if err != nil {
		return nil, err
	}

	resp, err := txmanager.WithTxResult(ctx, s.txManager.GetTx(), func(tx *sqlx.Tx) (*dto.CommitReservationResp, error) {
		var txns []*model.CreditTransaction
		for _, item := range items {
			w, err := s.walletRepo.FindByID(ctx, item.WalletID)
			if err != nil || w == nil {
				return nil, constant.ErrWalletNotFound
			}
			balBefore := w.Balance
			w.CommitReserved(item.Amount)
			if _, err := s.walletRepo.Save(ctx, tx, w); err != nil {
				return nil, err
			}
			note := "commit reservation"
			txns = append(txns, model.NewCreditTransaction(
				reservation.UserID, w.ID, w.WalletType,
				enum.TxTypeCommit, item.Amount, balBefore, w.Balance,
				&reservation.Uuid, nil, &req.IdempotencyKey, &note,
			))
		}

		if _, err := s.txRepo.SaveAll(ctx, tx, txns); err != nil {
			return nil, err
		}

		reservation.Commit()
		if _, err := s.reservationRepo.Save(ctx, tx, reservation); err != nil {
			return nil, err
		}

		return &dto.CommitReservationResp{
			Success:       true,
			TotalDeducted: reservation.TotalAmount,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	_ = s.idemStore.Set(ctx, req.IdempotencyKey, resp, 24*time.Hour)

	if s.publisher != nil {
		if pubErr := s.publisher.Publish(ctx, messaging.CreditCommitted{
			ReservationID: reservation.Uuid,
			UserID:        reservation.UserID,
			TotalDeducted: reservation.TotalAmount,
			Timestamp:     time.Now().UnixMilli(),
		}); pubErr != nil {
			logger.Logger.Error("failed to publish CreditCommitted", zap.Error(pubErr))
		}
	}

	return resp, nil
}
