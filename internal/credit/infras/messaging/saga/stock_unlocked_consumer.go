package saga

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"simple-securities/internal/credit/application/dto"
	"simple-securities/internal/credit/application/service"
	"simple-securities/internal/credit/domain/messaging"
	"simple-securities/pkg/kafka"
	"simple-securities/pkg/logger"
)

// StockUnlockedConsumer — Credit Service consumes "stock.unlocked" topic.
// Khi Personal đã insert unlock record thành công → Credit tự động commit reservation.
// Nếu commit fail → publish "credit.commit.failed" để Personal compensate.
type StockUnlockedConsumer struct {
	commitSvc   service.CommitReservationSvc
	rollbackSvc service.RollbackReservationSvc
	publisher   messaging.IEventPublisher
}

func NewStockUnlockedConsumer(
	commitSvc service.CommitReservationSvc,
	rollbackSvc service.RollbackReservationSvc,
	publisher messaging.IEventPublisher,
) *StockUnlockedConsumer {
	return &StockUnlockedConsumer{
		commitSvc:   commitSvc,
		rollbackSvc: rollbackSvc,
		publisher:   publisher,
	}
}

// Handle là EventHandler được inject vào kafka.Consumer.
func (c *StockUnlockedConsumer) Handle(ctx context.Context, key, raw []byte) error {
	var event kafka.Event
	if err := json.Unmarshal(raw, &event); err != nil {
		logger.Logger.Error("[saga] failed to unmarshal StockUnlocked", zap.Error(err))
		return nil // không retry parse error — đẩy sang DLQ nếu có
	}

	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return nil
	}

	var payload messaging.StockUnlocked
	if err := json.Unmarshal(dataBytes, &payload); err != nil {
		logger.Logger.Error("[saga] failed to unmarshal StockUnlocked payload", zap.Error(err))
		return nil
	}

	logger.Logger.Info("[saga] StockUnlocked received, committing reservation",
		zap.String("reservation_id", payload.ReservationID),
		zap.Uint64("user_id", payload.UserID),
	)

	_, err = c.commitSvc.Execute(ctx, &dto.CommitReservationReq{
		ReservationID:  payload.ReservationID,
		IdempotencyKey: payload.IdempotencyKey,
	})
	if err != nil {
		// Commit failed → publish CreditCommitFailed để Personal compensate
		logger.Logger.Error("[saga] commit reservation failed, publishing CreditCommitFailed",
			zap.String("reservation_id", payload.ReservationID),
			zap.Error(err),
		)

		_ = c.publisher.Publish(ctx, messaging.CreditCommitFailed{
			ReservationID:  payload.ReservationID,
			UserID:         payload.UserID,
			UnlockRecordID: payload.UnlockRecordID,
			Reason:         err.Error(),
			Timestamp:      time.Now().UnixMilli(),
		})

		// Rollback reservation để giải phóng reserved credits
		_, _ = c.rollbackSvc.Execute(ctx, &dto.RollbackReservationReq{
			ReservationID: payload.ReservationID,
			Reason:        "saga commit failed: " + err.Error(),
		})

		return nil // return nil để Kafka không retry vô hạn — đã compensate rồi
	}

	logger.Logger.Info("[saga] reservation committed successfully",
		zap.String("reservation_id", payload.ReservationID),
	)
	return nil
}
