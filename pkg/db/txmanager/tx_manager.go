package txmanager

import (
	"context"
	"fmt"
	"simple-securities/common/constants"
	"simple-securities/pkg/logger"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

type TxManager interface {
	GetTx() *txManager
	WithTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error
}

type txManager struct {
	db *sqlx.DB
}

func NewTxManager(db *sqlx.DB) TxManager {
	return &txManager{db: db}
}

func (m *txManager) GetTx() *txManager {
	return m
}

func (m *txManager) WithTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	start := time.Now()

	tx, err := m.db.BeginTxx(ctx, nil)
	logger.Info(ctx, "✅ [WithTxResult] Begin transaction")

	if err != nil {
		logger.Error(ctx, "🛑 [TxManager] Failed to begin transaction",
			zap.Any(constants.Error, err),
			zap.Duration(constants.Duration, time.Since(start)))
		return fmt.Errorf("begin tx failed: %w", err)
	}

	// Ensure rollback is called if the function panics or returns an error
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			elapsed := time.Since(start)
			logger.Error(ctx, "🔥 [TxManager] Transaction panic recovered - rolling back",
				zap.Any(constants.Panic, p),
				zap.Duration(constants.Duration, elapsed))
			panic(p) // re-throw panic after logging/rollback
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback() // Explicit rollback on error
		elapsed := time.Since(start)
		logger.Error(ctx, "⚠️ [TxManager] Transaction rolled back due to error",
			zap.Any(constants.Error, err),
			zap.Duration(constants.Duration, elapsed))
		return err
	}

	if err := tx.Commit(); err != nil {
		elapsed := time.Since(start)
		logger.Error(ctx, "🛑 [TxManager] Failed to commit transaction",
			zap.Any(constants.Error, err),
			zap.Duration(constants.Duration, elapsed))
		return fmt.Errorf("commit tx failed: %w", err)
	}

	elapsed := time.Since(start)
	logger.Info(ctx, "✅ [TxManager] Transaction committed successfully",
		zap.Duration(constants.Duration, elapsed))

	return nil
}

func WithTxResult[T any](
	ctx context.Context,
	m *txManager, // Truyền TxManager vào đây
	fn func(tx *sqlx.Tx) (T, error),
) (T, error) {
	start := time.Now()
	var zero T

	tx, err := m.db.BeginTxx(ctx, nil)
	logger.Info(ctx, "✅ [WithTxResult] Begin transaction")

	if err != nil {
		logger.Error(ctx, "🛑 [WithTxResult] Failed to begin",
			zap.Any(constants.Error, err),
			zap.Duration(constants.Duration, time.Since(start)))
		return zero, err
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			logger.Error(ctx, "🔥 [WithTxResult] Panic recovered",
				zap.Any(constants.Panic, p),
				zap.Duration(constants.Duration, time.Since(start)))
			panic(p)
		}
	}()

	res, err := fn(tx)
	if err != nil {
		tx.Rollback()
		logger.Error(ctx, "⚠️ [WithTxResult] Transaction rolled back",
			zap.Any(constants.Error, err),
			zap.Duration(constants.Duration, time.Since(start)))
		return zero, err
	}

	if err := tx.Commit(); err != nil {
		logger.Error(ctx, "🛑 [WithTxResult] Failed to commit",
			zap.Any(constants.Error, err),
			zap.Duration(constants.Duration, time.Since(start)))
		return zero, err
	}

	logger.Info(ctx, "✅ [WithTxResult] Success",
		zap.Duration(constants.Duration, time.Since(start)))

	return res, nil
}
