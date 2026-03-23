package saga

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"simple-securities/internal/credit/domain/messaging"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/logger"
)

// CreditCommitFailedConsumer — Personal Service consumes "credit.commit.failed".
// Compensating transaction: xoá unlock record đã insert, hoàn tác cho user biết.
type CreditCommitFailedConsumer struct {
	// unlockRepo là repository của Personal Service để xoá unlock record
	unlockRepo  UnlockRecordCompensator
	publisher   messaging.IEventPublisher
}

// UnlockRecordCompensator là interface Personal Service implement
// để tránh coupling Credit domain với Personal domain.
type UnlockRecordCompensator interface {
	DeleteByID(ctx context.Context, unlockRecordID string) error
}

func NewCreditCommitFailedConsumer(
	unlockRepo UnlockRecordCompensator,
	publisher messaging.IEventPublisher,
) *CreditCommitFailedConsumer {
	return &CreditCommitFailedConsumer{
		unlockRepo: unlockRepo,
		publisher:  publisher,
	}
}

func (c *CreditCommitFailedConsumer) Handle(ctx context.Context, key, raw []byte) error {
	var event kafka.Event
	if err := json.Unmarshal(raw, &event); err != nil {
		logger.Logger.Error("[saga] failed to unmarshal CreditCommitFailed", zap.Error(err))
		return nil
	}

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return nil
	}

	var payload messaging.CreditCommitFailed
	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		logger.Logger.Error("[saga] failed to unmarshal CreditCommitFailed payload", zap.Error(err))
		return nil
	}

	logger.Logger.Warn("[saga] CreditCommitFailed received, compensating unlock record",
		zap.String("reservation_id", payload.ReservationID),
		zap.String("unlock_record_id", payload.UnlockRecordID),
		zap.String("reason", payload.Reason),
	)

	// Compensating transaction: xoá unlock record đã insert
	if err := c.unlockRepo.DeleteByID(ctx, payload.UnlockRecordID); err != nil {
		logger.Logger.Error("[saga] failed to delete unlock record during compensation",
			zap.String("unlock_record_id", payload.UnlockRecordID),
			zap.Error(err),
		)
		// Trả error để Kafka retry (at-least-once delivery)
		return err
	}

	// Thông báo compensation hoàn tất (optional — cho audit/monitoring)
	_ = c.publisher.Publish(ctx, messaging.CreditRolledBack{
		ReservationID: payload.ReservationID,
		UserID:        payload.UserID,
		Reason:        "compensated: " + payload.Reason,
		Timestamp:     time.Now().UnixMilli(),
	})

	logger.Logger.Info("[saga] compensation completed",
		zap.String("unlock_record_id", payload.UnlockRecordID),
	)
	return nil
}
