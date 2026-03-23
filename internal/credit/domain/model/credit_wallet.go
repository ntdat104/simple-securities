package model

import (
	"time"

	"simple-securities/internal/credit/domain/enum"
	"simple-securities/pkg/uuid"
)

// CreditWallet represents a credit bucket per (user, wallet_type).
// PURCHASED wallets have nil ExpireAt — no expiry.
type CreditWallet struct {
	ID         uint64          `db:"id"`
	Uuid       string          `db:"uuid"`
	UserID     uint64          `db:"user_id"`
	WalletType enum.WalletType `db:"wallet_type"`
	Balance    int64           `db:"balance"`  // total; never negative
	Reserved   int64           `db:"reserved"` // soft-locked amount
	ExpireAt   *time.Time      `db:"expire_at"` // nil = no expiry
	CreatedAt  time.Time       `db:"created_at"`
	UpdatedAt  time.Time       `db:"updated_at"`
}

func (w CreditWallet) TableName() string { return "credit_wallets" }

func (w CreditWallet) Available() int64 { return w.Balance - w.Reserved }

func (w CreditWallet) IsExpired() bool {
	if w.ExpireAt == nil {
		return false
	}
	return time.Now().After(*w.ExpireAt)
}

func NewCreditWallet(userID uint64, walletType enum.WalletType, expireAt *time.Time) *CreditWallet {
	now := time.Now()
	return &CreditWallet{
		Uuid:       uuid.NewGoogleUUID(),
		UserID:     userID,
		WalletType: walletType,
		Balance:    0,
		Reserved:   0,
		ExpireAt:   expireAt,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// AddBalance tops up the wallet.
func (w *CreditWallet) AddBalance(amount int64) {
	w.Balance += amount
	w.UpdatedAt = time.Now()
}

// SoftReserve marks amount as reserved. Returns false if insufficient available credits.
func (w *CreditWallet) SoftReserve(amount int64) bool {
	if w.Available() < amount {
		return false
	}
	w.Reserved += amount
	w.UpdatedAt = time.Now()
	return true
}

// CommitReserved finalises a reservation: reduces balance + clears reserve.
func (w *CreditWallet) CommitReserved(amount int64) {
	w.Balance -= amount
	w.Reserved -= amount
	w.UpdatedAt = time.Now()
}

// ReleaseReserved cancels a reservation without touching balance.
func (w *CreditWallet) ReleaseReserved(amount int64) {
	w.Reserved -= amount
	w.UpdatedAt = time.Now()
}
