package messaging

import "fmt"

type CreditToppedUp struct {
	UserID         uint64 `json:"user_id"`
	WalletType     string `json:"wallet_type"`
	Amount         int64  `json:"amount"`
	NewBalance     int64  `json:"new_balance"`
	ReferenceID    string `json:"reference_id"`
	IdempotencyKey string `json:"idempotency_key"`
	Timestamp      int64  `json:"timestamp"`
}

func (e CreditToppedUp) EventName() string { return "credit.topped_up" }
func (e CreditToppedUp) EventKey() string  { return fmt.Sprintf("%d", e.UserID) }

type CreditReserved struct {
	ReservationID string `json:"reservation_id"`
	UserID        uint64 `json:"user_id"`
	TotalAmount   int64  `json:"total_amount"`
	StockSymbol   string `json:"stock_symbol"`
	RequestID     string `json:"request_id"`
	Timestamp     int64  `json:"timestamp"`
}

func (e CreditReserved) EventName() string { return "credit.reserved" }
func (e CreditReserved) EventKey() string  { return fmt.Sprintf("%d", e.UserID) }

type CreditCommitted struct {
	ReservationID string `json:"reservation_id"`
	UserID        uint64 `json:"user_id"`
	TotalDeducted int64  `json:"total_deducted"`
	Timestamp     int64  `json:"timestamp"`
}

func (e CreditCommitted) EventName() string { return "credit.committed" }
func (e CreditCommitted) EventKey() string  { return fmt.Sprintf("%d", e.UserID) }

type CreditRolledBack struct {
	ReservationID string `json:"reservation_id"`
	UserID        uint64 `json:"user_id"`
	Reason        string `json:"reason"`
	Timestamp     int64  `json:"timestamp"`
}

func (e CreditRolledBack) EventName() string { return "credit.rolled_back" }
func (e CreditRolledBack) EventKey() string  { return fmt.Sprintf("%d", e.UserID) }

// StockUnlocked — published by Personal Service after inserting unlock record.
// Credit Service consumes this to auto-commit the reservation.
type StockUnlocked struct {
	ReservationID  string `json:"reservation_id"`
	UserID         uint64 `json:"user_id"`
	StockSymbol    string `json:"stock_symbol"`
	UnlockRecordID string `json:"unlock_record_id"` // Personal's DB record ID for compensation
	IdempotencyKey string `json:"idempotency_key"`
	Timestamp      int64  `json:"timestamp"`
}

func (e StockUnlocked) EventName() string { return "stock.unlocked" }
func (e StockUnlocked) EventKey() string  { return fmt.Sprintf("%d", e.UserID) }

// CreditCommitFailed — published by Credit Service when auto-commit fails.
// Personal Service consumes this to compensate (delete unlock record).
type CreditCommitFailed struct {
	ReservationID  string `json:"reservation_id"`
	UserID         uint64 `json:"user_id"`
	UnlockRecordID string `json:"unlock_record_id"`
	Reason         string `json:"reason"`
	Timestamp      int64  `json:"timestamp"`
}

func (e CreditCommitFailed) EventName() string { return "credit.commit.failed" }
func (e CreditCommitFailed) EventKey() string  { return fmt.Sprintf("%d", e.UserID) }
