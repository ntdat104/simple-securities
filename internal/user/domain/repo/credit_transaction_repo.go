package repo

import (
	"context"

	"github.com/jmoiron/sqlx"

	"simple-securities/internal/user/domain/model"
)

// ICreditTransactionRepo writes to the immutable ledger.
type ICreditTransactionRepo interface {
	Save(ctx context.Context, tx *sqlx.Tx, txn *model.CreditTransaction) (*model.CreditTransaction, error)
	SaveAll(ctx context.Context, tx *sqlx.Tx, txns []*model.CreditTransaction) ([]*model.CreditTransaction, error)
}
