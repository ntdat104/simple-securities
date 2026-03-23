package model

import "simple-securities/internal/credit/domain/enum"

// CreditReservationItem is the per-wallet breakdown of a reservation.
type CreditReservationItem struct {
	ID            uint64          `db:"id"`
	ReservationID uint64          `db:"reservation_id"`
	WalletID      uint64          `db:"wallet_id"`
	WalletType    enum.WalletType `db:"wallet_type"`
	Amount        int64           `db:"amount"`
}

func (i CreditReservationItem) TableName() string { return "credit_reservation_items" }
