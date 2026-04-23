package repo

import (
	"context"

	"github.com/jmoiron/sqlx"

	"simple-securities/internal/user/domain/enum"
	"simple-securities/internal/user/domain/model"
)

// ICreditWalletRepo manages credit wallet persistence.
type ICreditWalletRepo interface {
	// FindByUserID returns all non-expired wallets for a user, ordered by expire_at ASC (PURCHASED last).
	FindByUserID(ctx context.Context, userID uint64) ([]*model.CreditWallet, error)

	// FindByUserIDAndType returns the wallet for a specific (user, walletType) pair.
	FindByUserIDAndType(ctx context.Context, userID uint64, walletType enum.WalletType) (*model.CreditWallet, error)

	// FindByID returns a single wallet by primary key.
	FindByID(ctx context.Context, id uint64) (*model.CreditWallet, error)

	// Save upserts a wallet within an optional transaction.
	Save(ctx context.Context, tx *sqlx.Tx, wallet *model.CreditWallet) (*model.CreditWallet, error)

	// SaveAll batch-saves wallets within an optional transaction.
	SaveAll(ctx context.Context, tx *sqlx.Tx, wallets []*model.CreditWallet) ([]*model.CreditWallet, error)
}
