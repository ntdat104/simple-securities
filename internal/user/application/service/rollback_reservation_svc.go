package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"simple-securities/internal/user/application/constant"
	"simple-securities/internal/user/application/dto"
	"simple-securities/internal/user/domain/enum"
	"simple-securities/internal/user/domain/messaging"
	"simple-securities/internal/user/domain/model"
	"simple-securities/internal/user/domain/repo"
	"simple-securities/pkg/db/txmanager"
	"simple-securities/pkg/logger"
)

type RollbackReservationSvc interface {
	Execute(ctx context.Context, req *dto.RollbackReservationReq) (*dto.RollbackReservationResp, error)
}

type rollbackReservationSvc struct {
	walletRepo          repo.ICreditWalletRepo
	txRepo              repo.ICreditTransactionRepo
	reservationRepo     repo.ICreditReservationRepo
	reservationItemRepo repo.ICreditReservationItemRepo
	txManager           txmanager.TxManager
	redisLock           DistributedLock
	publisher           messaging.IEventPublisher
}

func NewRollbackReservationSvc(
	walletRepo repo.ICreditWalletRepo,
	txRepo repo.ICreditTransactionRepo,
	reservationRepo repo.ICreditReservationRepo,
	reservationItemRepo repo.ICreditReservationItemRepo,
	txManager txmanager.TxManager,
	redisLock DistributedLock,
	publisher messaging.IEventPublisher,
) RollbackReservationSvc {
	return &rollbackReservationSvc{
		walletRepo:          walletRepo,
		txRepo:              txRepo,
		reservationRepo:     reservationRepo,
		reservationItemRepo: reservationItemRepo,
		txManager:           txManager,
		redisLock:           redisLock,
		publisher:           publisher,
	}
}

func (s *rollbackReservationSvc) Execute(ctx context.Context, req *dto.RollbackReservationReq) (*dto.RollbackReservationResp, error) {
	reservation, err := s.reservationRepo.FindByUuid(ctx, req.ReservationID)
	if err != nil || reservation == nil {
		return nil, constant.ErrReservationNotFound
	}
	// Idempotent: already rolled back is a success
	if reservation.Status == enum.ReservationRolledBack {
		return &dto.RollbackReservationResp{Success: true}, nil
	}
	if reservation.Status != enum.ReservationPending {
		return nil, constant.ErrReservationNotPending
	}

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

	_, err = txmanager.WithTxResult(ctx, s.txManager.GetTx(), func(tx *sqlx.Tx) (*dto.RollbackReservationResp, error) {
		var txns []*model.CreditTransaction
		for _, item := range items {
			w, err := s.walletRepo.FindByID(ctx, item.WalletID)
			if err != nil || w == nil {
				return nil, constant.ErrWalletNotFound
			}
			balBefore := w.Balance
			w.ReleaseReserved(item.Amount)
			if _, err := s.walletRepo.Save(ctx, tx, w); err != nil {
				return nil, err
			}
			note := "rollback: " + req.Reason
			txns = append(txns, model.NewCreditTransaction(
				reservation.UserID, w.ID, w.WalletType,
				enum.TxTypeRollback, item.Amount, balBefore, w.Balance,
				&reservation.Uuid, nil, nil, &note,
			))
		}

		if _, err := s.txRepo.SaveAll(ctx, tx, txns); err != nil {
			return nil, err
		}

		reservation.Rollback()
		if _, err := s.reservationRepo.Save(ctx, tx, reservation); err != nil {
			return nil, err
		}

		return &dto.RollbackReservationResp{Success: true}, nil
	})
	if err != nil {
		return nil, err
	}

	if s.publisher != nil {
		if pubErr := s.publisher.Publish(ctx, messaging.CreditRolledBack{
			ReservationID: reservation.Uuid,
			UserID:        reservation.UserID,
			Reason:        req.Reason,
			Timestamp:     time.Now().UnixMilli(),
		}); pubErr != nil {
			logger.Logger.Error("failed to publish CreditRolledBack", zap.Error(pubErr))
		}
	}

	return &dto.RollbackReservationResp{Success: true}, nil
}
