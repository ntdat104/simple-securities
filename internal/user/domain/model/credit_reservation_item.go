package model

import (
	"simple-securities/internal/user/domain/enum"
	"time"
)

// CreditReservationItem is the per-wallet breakdown of a reservation.
type CreditReservationItem struct {
	ID            uint64          `db:"id"`
	Uuid          string          `db:"uuid"`
	ReservationID uint64          `db:"reservation_id"`
	WalletID      uint64          `db:"wallet_id"`
	WalletType    enum.WalletType `db:"wallet_type"`
	Amount        int64           `db:"amount"`
	CreatedAt     time.Time       `db:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at"`
	CreatedBy     uint64          `db:"created_by"` // Can be 0 for self-registration or an admin ID
	UpdatedBy     uint64          `db:"updated_by"`
}

func (i CreditReservationItem) TableName() string { return "credit_reservation_items" }
