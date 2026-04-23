package model

import (
	"time"

	"simple-securities/internal/user/domain/enum"
	"simple-securities/pkg/uuid"
)

// CreditReservation is a pending soft-reservation for a stock unlock request.
type CreditReservation struct {
	ID             uint64                 `db:"id"`
	Uuid           string                 `db:"uuid"`
	UserID         uint64                 `db:"user_id"`
	TotalAmount    int64                  `db:"total_amount"`
	Status         enum.ReservationStatus `db:"status"`
	IdempotencyKey string                 `db:"idempotency_key"`
	StockSymbol    *string                `db:"stock_symbol"`
	RequestID      *string                `db:"request_id"`
	CommittedAt    *time.Time             `db:"committed_at"`
	RolledBackAt   *time.Time             `db:"rolled_back_at"`
	ExpiresAt      time.Time              `db:"expires_at"`
	CreatedAt      time.Time              `db:"created_at"`
	UpdatedAt      time.Time              `db:"updated_at"`
	CreatedBy      uint64                 `db:"created_by"` // Can be 0 for self-registration or an admin ID
	UpdatedBy      uint64                 `db:"updated_by"`
}

func (r CreditReservation) TableName() string { return "credit_reservations" }

func NewCreditReservation(
	userID uint64,
	totalAmount int64,
	idempotencyKey string,
	stockSymbol, requestID *string,
	ttl time.Duration,
) *CreditReservation {
	now := time.Now()
	return &CreditReservation{
		Uuid:           uuid.NewGoogleUUID(),
		UserID:         userID,
		TotalAmount:    totalAmount,
		Status:         enum.ReservationPending,
		IdempotencyKey: idempotencyKey,
		StockSymbol:    stockSymbol,
		RequestID:      requestID,
		ExpiresAt:      now.Add(ttl),
		CreatedAt:      now,
		UpdatedAt:      now,
		CreatedBy:      0,
		UpdatedBy:      0,
	}
}

func (r *CreditReservation) Commit() {
	now := time.Now()
	r.Status = enum.ReservationCommitted
	r.CommittedAt = &now
	r.UpdatedAt = now
}

func (r *CreditReservation) Rollback() {
	now := time.Now()
	r.Status = enum.ReservationRolledBack
	r.RolledBackAt = &now
	r.UpdatedAt = now
}
