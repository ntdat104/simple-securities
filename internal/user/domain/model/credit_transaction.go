package model

import (
	"time"

	"simple-securities/internal/user/domain/enum"
	"simple-securities/pkg/datetime"
	"simple-securities/pkg/uuid"
)

// CreditTransaction is the immutable ledger line for every credit movement.
type CreditTransaction struct {
	ID             uint64          `db:"id"`
	Uuid           string          `db:"uuid"`
	UserID         uint64          `db:"user_id"`
	WalletID       uint64          `db:"wallet_id"`
	WalletType     enum.WalletType `db:"wallet_type"`
	TxType         enum.TxType     `db:"tx_type"`
	Amount         int64           `db:"amount"` // always positive
	BalanceBefore  int64           `db:"balance_before"`
	BalanceAfter   int64           `db:"balance_after"`
	ReservationID  *string         `db:"reservation_id"` // uuid of reservation (nullable)
	ReferenceID    *string         `db:"reference_id"`
	IdempotencyKey *string         `db:"idempotency_key"`
	Note           *string         `db:"note"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
	CreatedBy      uint64          `db:"created_by"` // Can be 0 for self-registration or an admin ID
	UpdatedBy      uint64          `db:"updated_by"`
}

func (t CreditTransaction) TableName() string { return "credit_transactions" }

func NewCreditTransaction(
	userID, walletID uint64,
	walletType enum.WalletType,
	txType enum.TxType,
	amount, balanceBefore, balanceAfter int64,
	reservationID, referenceID, idempotencyKey, note *string,
) *CreditTransaction {
	now := datetime.Now()
	return &CreditTransaction{
		Uuid:           uuid.NewGoogleUUID(),
		UserID:         userID,
		WalletID:       walletID,
		WalletType:     walletType,
		TxType:         txType,
		Amount:         amount,
		BalanceBefore:  balanceBefore,
		BalanceAfter:   balanceAfter,
		ReservationID:  reservationID,
		ReferenceID:    referenceID,
		IdempotencyKey: idempotencyKey,
		Note:           note,
		CreatedAt:      now,
		UpdatedAt:      now,
		CreatedBy:      0,
		UpdatedBy:      0,
	}
}
